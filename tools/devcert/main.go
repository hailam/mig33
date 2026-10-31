// devcert creates private local-development TLS material. It never installs
// trust or overwrites an existing directory; production certificates are external.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type authority struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

func main() {
	out := flag.String("out", "data/local-tls", "new private output directory (must not exist)")
	hosts := flag.String("hosts", "localhost,127.0.0.1,::1", "comma-separated certificate DNS names or IPs")
	fixtures := flag.Bool("test-fixtures", false, "also generate expired, wrong-host and untrusted test certificates")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "devcert accepts flags only")
		os.Exit(1)
	}
	if err := generate(*out, strings.Split(*hosts, ","), *fixtures, time.Now()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Created local TLS certificates in", *out)
	fmt.Println("Trust only ca.pem on a dedicated development device; keep every private key local.")
}

func serial() (*big.Int, error) {
	return rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
}

func writePEM(directory, name, kind string, data []byte) error {
	file, err := os.OpenFile(filepath.Join(directory, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if err = pem.Encode(file, &pem.Block{Type: kind, Bytes: data}); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func saveKey(directory, name string, key *ecdsa.PrivateKey) error {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	return writePEM(directory, name, "PRIVATE KEY", der)
}

func makeAuthority(directory, prefix string, now time.Time) (*authority, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	number, err := serial()
	if err != nil {
		return nil, err
	}
	cert := &x509.Certificate{
		SerialNumber: number, Subject: pkix.Name{CommonName: "mig33 local development " + prefix},
		NotBefore: now.Add(-time.Hour), NotAfter: now.AddDate(1, 0, 0),
		IsCA: true, BasicConstraintsValid: true, MaxPathLenZero: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	if err = writePEM(directory, prefix+".pem", "CERTIFICATE", der); err != nil {
		return nil, err
	}
	if err = saveKey(directory, prefix+"-key.pem", key); err != nil {
		return nil, err
	}
	parsed, err := x509.ParseCertificate(der)
	return &authority{cert: parsed, key: key}, err
}

func issue(directory, prefix string, issuer *authority, hosts []string, before, after time.Time) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	number, err := serial()
	if err != nil {
		return err
	}
	cert := &x509.Certificate{
		SerialNumber: number, Subject: pkix.Name{CommonName: "mig33 local development server"},
		NotBefore: before, NotAfter: after,
		BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	for _, host := range hosts {
		if ip := net.ParseIP(host); ip != nil {
			cert.IPAddresses = append(cert.IPAddresses, ip)
		} else {
			cert.DNSNames = append(cert.DNSNames, host)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, cert, issuer.cert, &key.PublicKey, issuer.key)
	if err != nil {
		return err
	}
	if err = writePEM(directory, prefix+".pem", "CERTIFICATE", der); err != nil {
		return err
	}
	return saveKey(directory, prefix+"-key.pem", key)
}

func generate(directory string, hosts []string, fixtures bool, now time.Time) error {
	if len(hosts) == 0 {
		return fmt.Errorf("at least one certificate host is required")
	}
	for index, host := range hosts {
		host = strings.TrimSpace(host)
		if host == "" || strings.ContainsAny(host, "/\\*\r\n\x00 ") || (strings.Contains(host, ":") && net.ParseIP(host) == nil) {
			return fmt.Errorf("invalid certificate host %q: supply DNS names or IPs, not URLs or ports", host)
		}
		hosts[index] = host
	}
	if err := os.MkdirAll(filepath.Dir(directory), 0o700); err != nil {
		return err
	}
	if err := os.Mkdir(directory, 0o700); err != nil {
		return fmt.Errorf("create new TLS directory (existing material is never overwritten): %w", err)
	}
	ca, err := makeAuthority(directory, "ca", now)
	if err != nil {
		return err
	}
	if err = issue(directory, "server", ca, hosts, now.Add(-time.Hour), now.AddDate(0, 0, 30)); err != nil {
		return err
	}
	if !fixtures {
		return nil
	}
	if err = issue(directory, "wrong-host", ca, []string{"wrong-host.invalid"}, now.Add(-time.Hour), now.AddDate(0, 0, 30)); err != nil {
		return err
	}
	if err = issue(directory, "expired", ca, hosts, now.Add(-72*time.Hour), now.Add(-24*time.Hour)); err != nil {
		return err
	}
	untrusted, err := makeAuthority(directory, "untrusted-ca", now)
	if err != nil {
		return err
	}
	return issue(directory, "untrusted", untrusted, hosts, now.Add(-time.Hour), now.AddDate(0, 0, 30))
}
