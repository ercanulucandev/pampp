package core

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const caCommonName = "PAMPP Local CA"

// localCA is a per-installation certificate authority. Its root is added to the
// current user's Windows trust store, so every site certificate it signs is trusted.
type localCA struct {
	dir  string // data/ssl
	cert *x509.Certificate
	key  crypto.Signer
}

func (c *localCA) certPath() string     { return filepath.Join(c.dir, "ca.crt") }
func (c *localCA) keyPath() string      { return filepath.Join(c.dir, "ca.key") }
func (c *localCA) siteCertPath() string { return filepath.Join(c.dir, "sites.crt") }
func (c *localCA) siteKeyPath() string  { return filepath.Join(c.dir, "sites.key") }

// loadOrCreateCA reads the CA from disk, creating a new one on first use.
func loadOrCreateCA(dir string) (*localCA, error) {
	ca := &localCA{dir: dir}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if cert, key, err := readPair(ca.certPath(), ca.keyPath()); err == nil && time.Until(cert.NotAfter) > 30*24*time.Hour {
		ca.cert, ca.key = cert, key
		return ca, nil
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	host, _ := os.Hostname()
	tmpl := &x509.Certificate{
		SerialNumber:          randomSerial(),
		Subject:               pkix.Name{CommonName: caCommonName, Organization: []string{"PAMPP"}, OrganizationalUnit: []string{os.Getenv("USERNAME") + "@" + host}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	cert, _ := x509.ParseCertificate(der)
	if err := writePair(ca.certPath(), ca.keyPath(), der, key); err != nil {
		return nil, err
	}
	// Site certificates signed by an old CA are now worthless.
	os.Remove(ca.siteCertPath())
	os.Remove(ca.siteKeyPath())
	ca.cert, ca.key = cert, key
	return ca, nil
}

// siteCertificate returns a certificate covering every domain, reissuing it only
// when the domain list changed, the CA changed or it is close to expiry.
func (c *localCA) siteCertificate(domains []string) (certFile, keyFile string, err error) {
	names := []string{"localhost"}
	for _, d := range domains {
		names = append(names, d, "*."+d)
	}
	sort.Strings(names)

	if cert, _, err := readPair(c.siteCertPath(), c.siteKeyPath()); err == nil &&
		time.Until(cert.NotAfter) > 30*24*time.Hour &&
		cert.CheckSignatureFrom(c.cert) == nil &&
		sameStrings(sortedCopy(cert.DNSNames), names) {
		return c.siteCertPath(), c.siteKeyPath(), nil
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}
	tmpl := &x509.Certificate{
		SerialNumber: randomSerial(),
		Subject:      pkix.Name{CommonName: "PAMPP sites", Organization: []string{"PAMPP"}},
		DNSNames:     names,
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(0, 0, 397), // browsers reject leaf certs valid > 398 days
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, c.cert, &key.PublicKey, c.key)
	if err != nil {
		return "", "", err
	}
	if err := writePair(c.siteCertPath(), c.siteKeyPath(), der, key); err != nil {
		return "", "", err
	}
	return c.siteCertPath(), c.siteKeyPath(), nil
}

// thumbprint is the SHA-1 fingerprint Windows uses to identify certificates.
func (c *localCA) thumbprint() string {
	sum := sha1.Sum(c.cert.Raw)
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

func (c *localCA) pem() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.cert.Raw})
}

// trusted reports whether the CA is in the current user's Trusted Root store.
func (c *localCA) trusted() bool {
	return hiddenCmd("certutil", "-user", "-store", "Root", c.thumbprint()).Run() == nil
}

// trust adds the CA to the user's Trusted Root store. Windows shows its own
// confirmation dialog; no administrator rights are needed.
func (c *localCA) trust() error {
	out, err := hiddenCmd("certutil", "-user", "-addstore", "Root", c.certPath()).CombinedOutput()
	if err != nil || !c.trusted() {
		return fmt.Errorf("sertifika eklenemedi (iptal edilmiş olabilir): %s", lastLine(string(out)))
	}
	return nil
}

func (c *localCA) untrust() error {
	out, err := hiddenCmd("certutil", "-user", "-delstore", "Root", c.thumbprint()).CombinedOutput()
	if err != nil {
		return fmt.Errorf("sertifika kaldırılamadı: %s", lastLine(string(out)))
	}
	return nil
}

// appendToBundle adds the CA to PHP's CA bundle so curl/file_get_contents can call https://site.test.
func (c *localCA) appendToBundle(bundle string) {
	b, err := os.ReadFile(bundle)
	if err != nil || bytes.Contains(b, c.pem()) {
		return
	}
	f, err := os.OpenFile(bundle, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString("\n" + caCommonName + "\n")
	f.Write(c.pem())
}

// ---- helpers ----

func randomSerial() *big.Int {
	n, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	return n
}

func writePair(certPath, keyPath string, der []byte, key *ecdsa.PrivateKey) error {
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		return err
	}
	return os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644)
}

func readPair(certPath, keyPath string) (*x509.Certificate, crypto.Signer, error) {
	cb, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, err
	}
	kb, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, err
	}
	cblock, _ := pem.Decode(cb)
	kblock, _ := pem.Decode(kb)
	if cblock == nil || kblock == nil {
		return nil, nil, fmt.Errorf("geçersiz PEM")
	}
	cert, err := x509.ParseCertificate(cblock.Bytes)
	if err != nil {
		return nil, nil, err
	}
	key, err := x509.ParsePKCS8PrivateKey(kblock.Bytes)
	if err != nil {
		return nil, nil, err
	}
	signer, ok := key.(crypto.Signer)
	if !ok {
		return nil, nil, fmt.Errorf("desteklenmeyen anahtar")
	}
	return cert, signer, nil
}

func sortedCopy(s []string) []string {
	out := append([]string(nil), s...)
	sort.Strings(out)
	return out
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
