package wiring

import (
	"crypto/tls"
	"fmt"

	"github.com/hailam/mig33/internal/config"
)

// LoadTLS is called by an entrypoint before opening its graph or any listener.
// Certificates are shared by the secure listeners and reloaded on restart.
func LoadTLS(configuration config.TLS) (*tls.Config, error) {
	if configuration.CertFile == "" || configuration.KeyFile == "" {
		return nil, fmt.Errorf("secure listeners require tls.cert_file and tls.key_file")
	}
	certificate, err := tls.LoadX509KeyPair(configuration.CertFile, configuration.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load TLS certificate and key: %w", err)
	}
	return &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12}, nil
}
