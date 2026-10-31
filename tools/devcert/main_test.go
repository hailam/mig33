package main

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDevelopmentCertificatesAndNegativeFixtures(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tls")
	now := time.Now()
	if err := generate(dir, []string{"localhost", "127.0.0.1", "::1"}, true, now); err != nil {
		t.Fatal(err)
	}
	root, err := os.ReadFile(filepath.Join(dir, "ca.pem"))
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(root) {
		t.Fatal("invalid generated CA")
	}
	for _, name := range []string{"server", "wrong-host", "expired", "untrusted"} {
		pair, err := tls.LoadX509KeyPair(filepath.Join(dir, name+".pem"), filepath.Join(dir, name+"-key.pem"))
		if err != nil {
			t.Fatal(err)
		}
		for _, host := range []string{"localhost", "127.0.0.1", "::1"} {
			_, err = pair.Leaf.Verify(x509.VerifyOptions{Roots: pool, DNSName: host, CurrentTime: now})
			if (err == nil) != (name == "server") {
				t.Fatalf("%s for %s: %v", name, host, err)
			}
		}
		info, err := os.Stat(filepath.Join(dir, name+"-key.pem"))
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("private key mode: %v, %v", info, err)
		}
	}
	if err := generate(dir, []string{"localhost"}, false, now); err == nil {
		t.Fatal("existing material was overwritten")
	}
	if err := generate(filepath.Join(t.TempDir(), "invalid"), []string{"https://localhost"}, false, now); err == nil {
		t.Fatal("URL accepted as certificate identity")
	}
}
