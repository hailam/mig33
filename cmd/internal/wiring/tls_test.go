package wiring

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hailam/mig33/internal/adapters/httptransport"
	"github.com/hailam/mig33/internal/adapters/socket"
	"github.com/hailam/mig33/internal/bootstrap"
	"github.com/hailam/mig33/internal/config"
	"github.com/hailam/mig33/internal/controllers"
	"github.com/hailam/mig33/internal/protocol"
	"github.com/hailam/mig33/internal/testtls"
)

func TestLoadTLSValidatesIdentityBeforeStartup(t *testing.T) {
	identity := testtls.New(t)
	secure, err := LoadTLS(config.TLS{CertFile: identity.CertFile, KeyFile: identity.KeyFile})
	if err != nil {
		t.Fatal(err)
	}
	if secure.MinVersion != tls.VersionTLS12 || len(secure.Certificates) != 1 {
		t.Fatal("TLS policy missing")
	}
	badPEM := filepath.Join(t.TempDir(), "malformed.pem")
	if err := os.WriteFile(badPEM, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	other := testtls.New(t)
	for _, tc := range []config.TLS{
		{},
		{CertFile: identity.CertFile},
		{CertFile: "missing.pem", KeyFile: identity.KeyFile},
		{CertFile: badPEM, KeyFile: identity.KeyFile},
		{CertFile: identity.CertFile, KeyFile: other.KeyFile},
	} {
		if _, err := LoadTLS(tc); err == nil {
			t.Fatalf("invalid identity accepted: %+v", tc)
		}
	}
}

func TestFourTransportsShareOriginalSessionsAndGateway(t *testing.T) {
	cfg := configuration(t)
	identity := testtls.New(t)
	cfg.TLS.CertFile, cfg.TLS.KeyFile = identity.CertFile, identity.KeyFile
	secure, err := LoadTLS(cfg.TLS)
	if err != nil {
		t.Fatal(err)
	}
	graph, cleanup, err := Open(t.Context(), cfg, compositionLogger())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Error(err)
		}
	})
	controller := controllers.New(graph.HTTP)
	plainHTTP, err := httptransport.NewServer(&http.Server{Addr: "127.0.0.1:0", Handler: httptransport.New(controller)}, compositionLogger())
	if err != nil {
		t.Fatal(err)
	}
	https, err := httptransport.NewTLSServer(&http.Server{Addr: "127.0.0.1:0", Handler: httptransport.NewGateway(controller)}, secure, time.Second, compositionLogger())
	if err != nil {
		t.Fatal(err)
	}
	plainSocket, err := socket.NewListener(socket.New(graph.Native), "127.0.0.1:0", compositionLogger())
	if err != nil {
		t.Fatal(err)
	}
	tlsSocket, err := socket.NewTLSListener(socket.New(graph.Native), "127.0.0.1:0", secure, time.Second, compositionLogger())
	if err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{}, 4)
	component := func(name string, start func(context.Context) error, run func(context.Context) error, stop func(context.Context) error) bootstrap.Component {
		return bootstrap.Component{Name: name, Start: func(ctx context.Context) error {
			if err := start(ctx); err != nil {
				return err
			}
			ready <- struct{}{}
			return nil
		}, Run: run, Stop: stop}
	}
	stop := launch(t,
		bootstrap.Component{Name: "core", Run: graph.Core.Run},
		component("HTTP", plainHTTP.Start, plainHTTP.Run, plainHTTP.Stop),
		component("socket", plainSocket.Start, plainSocket.Run, plainSocket.Stop),
		component("HTTPS", https.Start, https.Run, https.Stop),
		component("TLS socket", tlsSocket.Start, tlsSocket.Run, tlsSocket.Stop),
	)
	for range 4 {
		select {
		case <-ready:
		case <-time.After(time.Second):
			t.Fatal("all four transports did not bind")
		}
	}
	transport := &http.Transport{TLSClientConfig: identity.Client}
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport, Timeout: time.Second}
	plainURL, secureURL := "http://"+plainHTTP.Addr().String(), "https://"+https.Addr().String()
	for _, origin := range []string{plainURL, secureURL} {
		response, err := client.Get(origin + "/healthz")
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("health on %s: %d", origin, response.StatusCode)
		}
		var request bytes.Buffer
		if err := protocol.Encode(&request, protocol.New(2, 17)); err != nil {
			t.Fatal(err)
		}
		response, err = client.Post(origin+"/", "application/octet-stream", &request)
		if err != nil {
			t.Fatal(err)
		}
		packet, decodeErr := protocol.Decode(response.Body)
		response.Body.Close()
		if response.StatusCode != 200 || decodeErr != nil || packet.Opcode != 3 || packet.Transaction != 17 || response.Header.Get("Content-Type") != "image/gif" {
			t.Fatalf("original gateway ping: %d %v %v", response.StatusCode, packet, decodeErr)
		}
	}
	for _, secureStream := range []bool{false, true} {
		var connection net.Conn
		if secureStream {
			connection, err = tls.DialWithDialer(&net.Dialer{Timeout: time.Second}, "tcp", tlsSocket.Addr().String(), identity.Client)
		} else {
			connection, err = net.DialTimeout("tcp", plainSocket.Addr().String(), time.Second)
		}
		if err != nil {
			t.Fatal(err)
		}
		connection.SetDeadline(time.Now().Add(2 * time.Second))
		username := "alice"
		if secureStream {
			username = "bob"
		}
		if err := protocol.Encode(connection, protocol.New(200, 1).SetText(5, username)); err != nil {
			t.Fatal(err)
		}
		challenge, err := protocol.Decode(connection)
		if err != nil || challenge.Opcode != 201 {
			t.Fatalf("challenge: %v %v", challenge, err)
		}
		value, _ := challenge.Text(1)
		sid, _ := challenge.Text(2)
		if err := protocol.Encode(connection, protocol.New(202, 2).SetInt(1, protocol.JavaHash(value+username+"-demo"))); err != nil {
			t.Fatal(err)
		}
		login, err := protocol.Decode(connection)
		if err != nil || login.Opcode != 203 {
			t.Fatalf("login: %v %v", login, err)
		}
		// Both HTTP variants accept the same original SID while its socket
		// remains open. The HTTPS gateway needs no new login/session format.
		for _, origin := range []string{plainURL, secureURL} {
			response, err := client.Get(origin + "/" + sid)
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(response.Body)
			response.Body.Close()
			if err != nil || response.StatusCode != 200 {
				t.Fatalf("shared original SID: %d %s %v", response.StatusCode, body, err)
			}
		}
		request, _ := http.NewRequestWithContext(t.Context(), "GET", plainURL+"/api/chat/session", nil)
		request.Header.Set("sid", sid)
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil || response.StatusCode != 200 || !bytes.Contains(body, []byte(`"username":"`+username+`"`)) {
			t.Fatalf("same graph: %d %s %v", response.StatusCode, body, err)
		}
		connection.Close()
	}
	// HTTPS exposes only gateway + health; browser/API/direct resource URLs
	// must not inherit the legacy HTTP origin policy or become redirects.
	for _, path := range []string{"/api/catalog", "/api/chat/session", "/api/contacts", "/api/photos", "/assets/photos/test.png", "/member2/t.php", "/operator", "/operator/", "/sites/index", "/sites/resources/language/v4.6/en-US"} {
		response, err := client.Get(secureURL + path)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusNotFound || response.Header.Get("Location") != "" {
			t.Errorf("HTTPS route %s exposed: %d", path, response.StatusCode)
		}
	}
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{"PUT", "/", "", 405}, {"POST", "/", "invalid", 400}, {"GET", "/", "", 401}, {"HEAD", "/healthz", "", 405},
	} {
		for _, origin := range []string{plainURL, secureURL} {
			request, _ := http.NewRequestWithContext(t.Context(), tc.method, origin+tc.path, bytes.NewBufferString(tc.body))
			response, err := client.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != tc.status {
				t.Errorf("%s %s on %s: %d want %d", tc.method, tc.path, origin, response.StatusCode, tc.status)
			}
		}
	}
	stop()
	for _, address := range []net.Addr{plainHTTP.Addr(), https.Addr(), plainSocket.Addr(), tlsSocket.Addr()} {
		connection, err := net.DialTimeout("tcp", address.String(), time.Second)
		if err == nil {
			connection.Close()
			t.Errorf("listener %s survived shutdown", address)
		}
	}
}

func TestTLSListenerBindFailureRollsBackEarlierTransports(t *testing.T) {
	identity := testtls.New(t)
	graph, cleanup, err := Open(t.Context(), configuration(t), compositionLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	plainHTTP, err := httptransport.NewServer(&http.Server{Addr: "127.0.0.1:0", Handler: http.NotFoundHandler()}, compositionLogger())
	if err != nil {
		t.Fatal(err)
	}
	plainSocket, err := socket.NewListener(socket.New(graph.Native), "127.0.0.1:0", compositionLogger())
	if err != nil {
		t.Fatal(err)
	}
	https, err := httptransport.NewTLSServer(&http.Server{Addr: "127.0.0.1:0", Handler: http.NotFoundHandler()}, identity.Server, time.Second, compositionLogger())
	if err != nil {
		t.Fatal(err)
	}
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	last, err := socket.NewTLSListener(socket.New(graph.Native), occupied.Addr().String(), identity.Server, time.Second, compositionLogger())
	if err != nil {
		t.Fatal(err)
	}
	notRun := func(context.Context) error {
		t.Error("worker started before all four listeners were ready")
		return nil
	}
	err = bootstrap.Run(t.Context(), time.Second, compositionLogger(),
		bootstrap.Component{Name: "HTTP", Start: plainHTTP.Start, Run: notRun, Stop: plainHTTP.Stop},
		bootstrap.Component{Name: "socket", Start: plainSocket.Start, Run: notRun, Stop: plainSocket.Stop},
		bootstrap.Component{Name: "HTTPS", Start: https.Start, Run: notRun, Stop: https.Stop},
		bootstrap.Component{Name: "TLS socket", Start: last.Start, Run: notRun, Stop: last.Stop},
	)
	if err == nil {
		t.Fatal("occupied secure listener was accepted")
	}
	for _, address := range []net.Addr{plainHTTP.Addr(), plainSocket.Addr(), https.Addr()} {
		connection, err := net.DialTimeout("tcp", address.String(), time.Second)
		if err == nil {
			connection.Close()
			t.Errorf("listener %s leaked during startup rollback", address)
		}
	}
}
