// Package config loads application defaults, YAML overlays and environment
// overrides without depending on a transport or persistence implementation.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/hailam/mig33/configs"
)

type Config struct {
	Environment     string        `yaml:"-"`
	PublicURL       string        `yaml:"public_url"`
	Database        Database      `yaml:"database"`
	Resources       Resources     `yaml:"resources"`
	HTTP            HTTP          `yaml:"http"`
	Socket          Socket        `yaml:"socket"`
	TLS             TLS           `yaml:"tls"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
	Operators       []string      `yaml:"operators"`
}

type Database struct {
	Path string `yaml:"path"`
}

type Resources struct {
	// Directory contains optional original-client download resources. An empty
	// path disables them; missing files do not prevent core services starting.
	Directory string `yaml:"directory"`
}

type HTTP struct {
	Listen            string        `yaml:"listen"`
	TLSListen         string        `yaml:"tls_listen"`
	ReadHeaderTimeout time.Duration `yaml:"read_header_timeout"`
	ReadTimeout       time.Duration `yaml:"read_timeout"`
	WriteTimeout      time.Duration `yaml:"write_timeout"`
	IdleTimeout       time.Duration `yaml:"idle_timeout"`
	MaxHeaderBytes    int           `yaml:"max_header_bytes"`
}

type Socket struct {
	Listen    string `yaml:"listen"`
	TLSListen string `yaml:"tls_listen"`
}

type TLS struct {
	CertFile         string        `yaml:"cert_file"`
	KeyFile          string        `yaml:"key_file"`
	HandshakeTimeout time.Duration `yaml:"handshake_timeout"`
}

type LoadOptions struct {
	// Directory defaults to configs. An explicitly supplied directory must
	// contain app.yaml; absent default directories use the embedded defaults.
	Directory string
	// DirectorySet makes an explicitly empty selection override MIG33_CONFIG_DIR.
	DirectorySet bool
	// Environment selects <environment>.yaml. An unset value uses MIG33_ENV.
	Environment string
	// EnvironmentSet permits an explicit empty value to disable MIG33_ENV.
	EnvironmentSet bool
	// Overlays are additional names, applied in this order after Environment.
	Overlays []string
	// Environ defaults to os.Environ. An empty non-nil slice disables overrides.
	Environ []string
	// Overrides are explicit CLI values, applied before final validation.
	Overrides Overrides
}

type Overrides struct {
	PublicURL, DatabasePath, HTTPListen, SocketListen       *string
	HTTPTLSListen, SocketTLSListen, TLSCertFile, TLSKeyFile *string
	Operators                                               *[]string
}

func (overrides Overrides) apply(config *Config) {
	for _, field := range []struct{ source, target *string }{
		{overrides.PublicURL, &config.PublicURL},
		{overrides.DatabasePath, &config.Database.Path},
		{overrides.HTTPListen, &config.HTTP.Listen},
		{overrides.SocketListen, &config.Socket.Listen},
		{overrides.HTTPTLSListen, &config.HTTP.TLSListen},
		{overrides.SocketTLSListen, &config.Socket.TLSListen},
		{overrides.TLSCertFile, &config.TLS.CertFile},
		{overrides.TLSKeyFile, &config.TLS.KeyFile},
	} {
		if field.source != nil {
			*field.target = *field.source
		}
	}
	if overrides.Operators != nil {
		config.Operators = append([]string{}, (*overrides.Operators)...)
	}
}

var overlayName = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

// Load applies embedded app.yaml, on-disk app.yaml, the selected environment,
// additional overlays, MIG33_* variables, then explicit overrides. The merged
// result is validated only after higher-precedence values have been applied.
func Load(options LoadOptions) (Config, error) {
	var result Config
	if err := decode(configs.Defaults, &result); err != nil {
		return Config{}, fmt.Errorf("embedded app.yaml: %w", err)
	}
	environment, err := environmentMap(options.Environ)
	if err != nil {
		return Config{}, err
	}
	directory := options.Directory
	if directory == "" && !options.DirectorySet {
		directory = environment["MIG33_CONFIG_DIR"]
	}
	explicitDirectory := directory != ""
	if directory == "" {
		directory = "configs"
	}
	if err := loadFile(filepath.Join(directory, "app.yaml"), &result); err != nil {
		if explicitDirectory || !errors.Is(err, os.ErrNotExist) {
			return Config{}, err
		}
	}
	name := options.Environment
	if name == "" && !options.EnvironmentSet {
		name = environment["MIG33_ENV"]
	}
	names := append([]string(nil), options.Overlays...)
	if name != "" {
		names = append([]string{name}, names...)
	}
	for _, overlay := range names {
		if !overlayName.MatchString(overlay) || overlay == "app" {
			return Config{}, fmt.Errorf("invalid overlay name %q: use a lowercase name other than app", overlay)
		}
		if err := loadFile(filepath.Join(directory, overlay+".yaml"), &result); err != nil {
			return Config{}, err
		}
	}
	result.Environment = name
	if err := applyEnvironment(&result, environment); err != nil {
		return Config{}, err
	}
	options.Overrides.apply(&result)
	if err := result.Validate(); err != nil {
		return Config{}, err
	}
	return result, nil
}

func loadFile(path string, config *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read configuration %s: %w", path, err)
	}
	if err := decode(data, config); err != nil {
		return fmt.Errorf("configuration %s: %w", path, err)
	}
	return nil
}

func decode(data []byte, config *Config) error {
	// Refuse nulls rather than silently keeping an inherited value. This also
	// catches accidentally empty YAML values such as "database: ".
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return err
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("configuration must be a YAML mapping")
	}
	var check func(*yaml.Node) error
	check = func(node *yaml.Node) error {
		if node.Tag == "!!null" {
			return fmt.Errorf("null configuration value at line %d", node.Line)
		}
		for _, child := range node.Content {
			if err := check(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := check(&document); err != nil {
		return err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(config); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return err
		}
		return fmt.Errorf("configuration must contain exactly one YAML document")
	}
	return nil
}

func environmentMap(environ []string) (map[string]string, error) {
	if environ == nil {
		environ = os.Environ()
	}
	result := make(map[string]string)
	for _, entry := range environ {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || !strings.HasPrefix(key, "MIG33_") {
			continue
		}
		switch key {
		case "MIG33_CONFIG_DIR", "MIG33_ENV", "MIG33_PUBLIC_URL", "MIG33_DATABASE_PATH", "MIG33_RESOURCES_DIRECTORY",
			"MIG33_HTTP_LISTEN", "MIG33_HTTP_READ_HEADER_TIMEOUT", "MIG33_HTTP_READ_TIMEOUT",
			"MIG33_HTTP_WRITE_TIMEOUT", "MIG33_HTTP_IDLE_TIMEOUT", "MIG33_HTTP_MAX_HEADER_BYTES",
			"MIG33_SOCKET_LISTEN", "MIG33_SHUTDOWN_TIMEOUT", "MIG33_OPERATORS",
			"MIG33_HTTP_TLS_LISTEN", "MIG33_SOCKET_TLS_LISTEN", "MIG33_TLS_CERT_FILE", "MIG33_TLS_KEY_FILE", "MIG33_TLS_HANDSHAKE_TIMEOUT":
			result[key] = value
		default:
			return nil, fmt.Errorf("unknown configuration environment variable %s", key)
		}
	}
	return result, nil
}

func applyEnvironment(config *Config, environment map[string]string) error {
	stringsByKey := map[string]*string{
		"MIG33_PUBLIC_URL":          &config.PublicURL,
		"MIG33_DATABASE_PATH":       &config.Database.Path,
		"MIG33_HTTP_LISTEN":         &config.HTTP.Listen,
		"MIG33_SOCKET_LISTEN":       &config.Socket.Listen,
		"MIG33_HTTP_TLS_LISTEN":     &config.HTTP.TLSListen,
		"MIG33_SOCKET_TLS_LISTEN":   &config.Socket.TLSListen,
		"MIG33_TLS_CERT_FILE":       &config.TLS.CertFile,
		"MIG33_TLS_KEY_FILE":        &config.TLS.KeyFile,
		"MIG33_RESOURCES_DIRECTORY": &config.Resources.Directory,
	}
	for key, target := range stringsByKey {
		if value, ok := environment[key]; ok {
			*target = value
		}
	}
	durations := map[string]*time.Duration{
		"MIG33_HTTP_READ_HEADER_TIMEOUT": &config.HTTP.ReadHeaderTimeout,
		"MIG33_HTTP_READ_TIMEOUT":        &config.HTTP.ReadTimeout,
		"MIG33_HTTP_WRITE_TIMEOUT":       &config.HTTP.WriteTimeout,
		"MIG33_HTTP_IDLE_TIMEOUT":        &config.HTTP.IdleTimeout,
		"MIG33_SHUTDOWN_TIMEOUT":         &config.ShutdownTimeout,
		"MIG33_TLS_HANDSHAKE_TIMEOUT":    &config.TLS.HandshakeTimeout,
	}
	for key, target := range durations {
		if value, ok := environment[key]; ok {
			parsed, err := time.ParseDuration(value)
			if err != nil {
				return fmt.Errorf("%s must be a duration such as 5s: %w", key, err)
			}
			*target = parsed
		}
	}
	if value, ok := environment["MIG33_HTTP_MAX_HEADER_BYTES"]; ok {
		number, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("MIG33_HTTP_MAX_HEADER_BYTES must be an integer: %w", err)
		}
		config.HTTP.MaxHeaderBytes = number
	}
	if value, ok := environment["MIG33_OPERATORS"]; ok {
		config.Operators = SplitOperators(value)
	}
	return nil
}

// SplitOperators preserves an explicit empty value, which disables the
// dashboard even when an overlay configured an allowlist.
func SplitOperators(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	names := strings.Split(value, ",")
	for i := range names {
		names[i] = strings.TrimSpace(names[i])
	}
	return names
}

func (config Config) Validate() error {
	u, err := url.Parse(config.PublicURL)
	if err != nil || u.Scheme != "http" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return fmt.Errorf("public_url must be an absolute http://host[:port] origin for the legacy client")
	}
	if strings.TrimSpace(config.Database.Path) == "" || strings.ContainsRune(config.Database.Path, 0) {
		return fmt.Errorf("database.path must not be empty or contain NUL")
	}
	for name, value := range map[string]time.Duration{
		"http.read_header_timeout": config.HTTP.ReadHeaderTimeout,
		"http.read_timeout":        config.HTTP.ReadTimeout,
		"http.write_timeout":       config.HTTP.WriteTimeout,
		"http.idle_timeout":        config.HTTP.IdleTimeout,
		"shutdown_timeout":         config.ShutdownTimeout,
		"tls.handshake_timeout":    config.TLS.HandshakeTimeout,
	} {
		if value <= 0 {
			return fmt.Errorf("%s must be positive", name)
		}
	}
	if config.HTTP.MaxHeaderBytes < 1024 || config.HTTP.MaxHeaderBytes > 16*1024*1024 {
		return fmt.Errorf("http.max_header_bytes must be between 1024 and 16777216")
	}
	if (config.TLS.CertFile == "") != (config.TLS.KeyFile == "") {
		return fmt.Errorf("tls.cert_file and tls.key_file must be configured together")
	}
	for _, name := range config.Operators {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(name) != name || strings.ContainsAny(name, ",\r\n\x00") {
			return fmt.Errorf("operators must contain nonempty account names")
		}
	}
	return nil
}

// ValidateListener is called only for transports enabled by the entrypoint.
func ValidateListener(name, address string) error {
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("%s must be host:port: %w", name, err)
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 0 || number > 65535 {
		return fmt.Errorf("%s port must be between 0 and 65535", name)
	}
	return nil
}
