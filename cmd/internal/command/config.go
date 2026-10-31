// Package command shares process and configuration plumbing between executables.
// Each executable selects its own flags, capabilities and application actions.
package command

import (
	"flag"
	"fmt"
	"io"

	"github.com/hailam/mig33/internal/config"
)

type overlayFlags []string

func (flags *overlayFlags) String() string         { return fmt.Sprint([]string(*flags)) }
func (flags *overlayFlags) Set(value string) error { *flags = append(*flags, value); return nil }

// Flags registers common configuration options. Entrypoints explicitly add
// their listeners or action flags before parsing.
type Flags struct {
	Set                                                        *flag.FlagSet
	directory, environment, publicURL, databasePath, operators *string
	httpAddress, socketAddress                                 *string
	httpsAddress, tlsSocketAddress, tlsCertFile, tlsKeyFile    *string
	overlays                                                   overlayFlags
}

func NewFlags(name string, stderr io.Writer) *Flags {
	set := flag.NewFlagSet(name, flag.ContinueOnError)
	set.SetOutput(stderr)
	f := &Flags{Set: set}
	f.directory = set.String("config-dir", "", "YAML configuration directory (default configs or MIG33_CONFIG_DIR)")
	f.environment = set.String("env", "", "environment overlay name (default MIG33_ENV)")
	set.Var(&f.overlays, "overlay", "additional YAML overlay name; repeat to apply in order")
	f.publicURL = set.String("public-url", "", "override HTTP origin advertised to the original client")
	f.databasePath = set.String("db", "", "override SQLite path; :memory: for temporary state")
	f.operators = set.String("operators", "", "override comma-separated operator accounts; empty disables dashboard")
	return f
}

func (f *Flags) HTTP() {
	f.httpAddress = f.Set.String("http", "", "override HTTP listen address")
	f.httpsAddress = f.Set.String("https", "", "optional HTTPS native gateway listen address; empty disables")
	f.tls()
}

func (f *Flags) Socket() {
	f.socketAddress = f.Set.String("tcp", "", "override native socket listen address")
	f.tlsSocketAddress = f.Set.String("tls-tcp", "", "optional TLS native socket listen address; empty disables")
	f.tls()
}

func (f *Flags) tls() {
	if f.tlsCertFile == nil {
		f.tlsCertFile = f.Set.String("tls-cert", "", "TLS certificate chain PEM file")
		f.tlsKeyFile = f.Set.String("tls-key", "", "TLS private key PEM file")
	}
}

// Parse checks command syntax before reading configuration. Entrypoints can
// validate their own actions before calling Load.
func (f *Flags) Parse(arguments []string) error {
	if err := f.Set.Parse(arguments); err != nil {
		return err
	}
	if f.Set.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", f.Set.Args())
	}
	return nil
}

// Load preserves explicit empty overrides and validates only selected listener
// addresses, before any application resources are constructed.
func (f *Flags) Load() (config.Config, error) {
	if !f.Set.Parsed() {
		return config.Config{}, fmt.Errorf("parse command flags before loading configuration")
	}
	options := config.LoadOptions{Directory: *f.directory, Environment: *f.environment, Overlays: f.overlays}
	f.Set.Visit(func(value *flag.Flag) {
		switch value.Name {
		case "config-dir":
			options.DirectorySet = true
		case "env":
			options.EnvironmentSet = true
		case "http":
			options.Overrides.HTTPListen = f.httpAddress
		case "tcp":
			options.Overrides.SocketListen = f.socketAddress
		case "https":
			options.Overrides.HTTPTLSListen = f.httpsAddress
		case "tls-tcp":
			options.Overrides.SocketTLSListen = f.tlsSocketAddress
		case "tls-cert":
			options.Overrides.TLSCertFile = f.tlsCertFile
		case "tls-key":
			options.Overrides.TLSKeyFile = f.tlsKeyFile
		case "public-url":
			options.Overrides.PublicURL = f.publicURL
		case "db":
			options.Overrides.DatabasePath = f.databasePath
		case "operators":
			names := config.SplitOperators(*f.operators)
			options.Overrides.Operators = &names
		}
	})
	configuration, err := config.Load(options)
	if err != nil {
		return config.Config{}, err
	}
	if f.httpAddress != nil {
		if err := config.ValidateListener("http.listen", configuration.HTTP.Listen); err != nil {
			return config.Config{}, err
		}
	}
	if f.socketAddress != nil {
		if err := config.ValidateListener("socket.listen", configuration.Socket.Listen); err != nil {
			return config.Config{}, err
		}
	}
	for _, listener := range []struct {
		selected      *string
		name, address string
	}{
		{f.httpsAddress, "http.tls_listen", configuration.HTTP.TLSListen},
		{f.tlsSocketAddress, "socket.tls_listen", configuration.Socket.TLSListen},
	} {
		if listener.selected == nil || listener.address == "" {
			continue
		}
		if err := config.ValidateListener(listener.name, listener.address); err != nil {
			return config.Config{}, err
		}
		if configuration.TLS.CertFile == "" || configuration.TLS.KeyFile == "" {
			return config.Config{}, fmt.Errorf("%s requires tls.cert_file and tls.key_file", listener.name)
		}
	}
	return configuration, nil
}
