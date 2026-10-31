package config

import (
	"testing"
	"time"
)

func TestTLSDefaultsAndOverlayEnvironmentOverrides(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "app", "{}")
	defaults, err := Load(LoadOptions{Directory: dir, Environ: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if defaults.HTTP.TLSListen != "" || defaults.Socket.TLSListen != "" || defaults.TLS.CertFile != "" || defaults.TLS.KeyFile != "" || defaults.TLS.HandshakeTimeout != 5*time.Second {
		t.Fatalf("secure listeners must default off: %+v", defaults)
	}
	writeConfig(t, dir, "secure", "http:\n  tls_listen: 127.0.0.1:8443\nsocket:\n  tls_listen: 127.0.0.1:9120\ntls:\n  cert_file: overlay.pem\n  key_file: overlay-key.pem\n  handshake_timeout: 3s\n")
	https := ""
	loaded, err := Load(LoadOptions{Directory: dir, Environment: "secure", Environ: []string{
		"MIG33_HTTP_TLS_LISTEN=127.0.0.1:9443", "MIG33_SOCKET_TLS_LISTEN=127.0.0.1:9220",
		"MIG33_TLS_CERT_FILE=environment.pem", "MIG33_TLS_KEY_FILE=environment-key.pem", "MIG33_TLS_HANDSHAKE_TIMEOUT=2s",
	}, Overrides: Overrides{HTTPTLSListen: &https}})
	if err != nil {
		t.Fatal(err)
	}
	if loaded.HTTP.TLSListen != "" || loaded.Socket.TLSListen != "127.0.0.1:9220" || loaded.TLS.CertFile != "environment.pem" || loaded.TLS.KeyFile != "environment-key.pem" || loaded.TLS.HandshakeTimeout != 2*time.Second {
		t.Fatalf("TLS precedence lost: %+v", loaded)
	}
	if loaded.HTTP.Listen != defaults.HTTP.Listen || loaded.Socket.Listen != defaults.Socket.Listen || loaded.PublicURL != defaults.PublicURL {
		t.Fatal("TLS configuration changed legacy transport defaults")
	}
}

func TestInvalidTLSConfiguration(t *testing.T) {
	for _, yaml := range []string{
		"tls:\n  cert_file: only-cert.pem\n", "tls:\n  key_file: only-key.pem\n", "tls:\n  handshake_timeout: 0s\n",
	} {
		dir := t.TempDir()
		writeConfig(t, dir, "app", yaml)
		if _, err := Load(LoadOptions{Directory: dir, Environ: []string{}}); err == nil {
			t.Fatalf("invalid TLS accepted: %s", yaml)
		}
	}
}
