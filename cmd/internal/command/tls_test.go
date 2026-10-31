package command

import (
	"io"
	"strings"
	"testing"
)

func TestSecureListenerFlagsAndValidation(t *testing.T) {
	clearConfigEnvironment(t)
	t.Chdir(t.TempDir())
	for _, tc := range []struct {
		name      string
		configure func(*Flags)
		arguments []string
		failure   string
	}{
		{"all listeners", func(f *Flags) { f.HTTP(); f.Socket() }, []string{"-https=127.0.0.1:8443", "-tls-tcp=127.0.0.1:9120", "-tls-cert=cert.pem", "-tls-key=key.pem"}, ""},
		{"missing identity", (*Flags).HTTP, []string{"-https=127.0.0.1:8443"}, "requires tls.cert_file"},
		{"bad HTTPS address", (*Flags).HTTP, []string{"-https=invalid", "-tls-cert=cert.pem", "-tls-key=key.pem"}, "http.tls_listen"},
		{"bad TLS socket address", (*Flags).Socket, []string{"-tls-tcp=invalid", "-tls-cert=cert.pem", "-tls-key=key.pem"}, "socket.tls_listen"},
		{"explicit disable", (*Flags).HTTP, []string{"-https="}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			flags := NewFlags("test", io.Discard)
			tc.configure(flags)
			if err := flags.Parse(tc.arguments); err != nil {
				t.Fatal(err)
			}
			cfg, err := flags.Load()
			if tc.failure != "" {
				if err == nil || !strings.Contains(err.Error(), tc.failure) {
					t.Fatalf("wanted %q, got %v", tc.failure, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.name == "all listeners" && (cfg.HTTP.TLSListen != "127.0.0.1:8443" || cfg.Socket.TLSListen != "127.0.0.1:9120" || cfg.TLS.CertFile != "cert.pem" || cfg.TLS.KeyFile != "key.pem") {
				t.Fatalf("CLI values lost: %+v", cfg)
			}
		})
	}
}

func TestOnlySelectedSecureCapabilitiesAreLoaded(t *testing.T) {
	clearConfigEnvironment(t)
	t.Chdir(t.TempDir())
	t.Setenv("MIG33_SOCKET_TLS_LISTEN", "invalid-unused")
	flags := NewFlags("http", io.Discard)
	flags.HTTP()
	if err := flags.Parse(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := flags.Load(); err != nil {
		t.Fatal(err)
	}
	if err := flags.Parse([]string{"-tls-tcp=127.0.0.1:9120"}); err == nil {
		t.Fatal("HTTP command exposed socket flag")
	}
	common := NewFlags("cli", io.Discard)
	if err := common.Parse([]string{"-tls-cert=unused.pem"}); err == nil {
		t.Fatal("non-server command exposed TLS flags")
	}
}
