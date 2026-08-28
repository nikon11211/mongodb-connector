package mongodb

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTLSConfigToTLSConfig_Nil(t *testing.T) {
	var tc *TLSConfig
	cfg, err := tc.ToTLSConfig()
	require.NoError(t, err)
	assert.Nil(t, cfg)
}

func TestTLSConfigToTLSConfig_Disabled(t *testing.T) {
	tc := &TLSConfig{}
	cfg, err := tc.ToTLSConfig()
	require.NoError(t, err)
	assert.Nil(t, cfg)
}

func TestTLSConfigToTLSConfig_EnabledWithoutFiles(t *testing.T) {
	tc := &TLSConfig{Enabled: true, InsecureSkipVerify: true}
	cfg, err := tc.ToTLSConfig()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.True(t, cfg.InsecureSkipVerify)
	assert.Nil(t, cfg.RootCAs)
	assert.Empty(t, cfg.Certificates)
}

func TestTLSConfigToTLSConfig_CAFile(t *testing.T) {
	caFile := writePEMFile(t, "ca.pem", generateCACertPEM(t))
	tc := &TLSConfig{Enabled: true, CAFile: caFile}
	cfg, err := tc.ToTLSConfig()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.NotNil(t, cfg.RootCAs)
}

func TestTLSConfigToTLSConfig_CAFileReadError(t *testing.T) {
	tc := &TLSConfig{Enabled: true, CAFile: filepath.Join(t.TempDir(), "missing.pem")}
	cfg, err := tc.ToTLSConfig()
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to read CA certificate")
	assert.Nil(t, cfg)
}

func TestTLSConfigToTLSConfig_CAFileParseError(t *testing.T) {
	badFile := writePEMFile(t, "bad.pem", []byte("not a valid PEM certificate"))
	tc := &TLSConfig{Enabled: true, CAFile: badFile}
	cfg, err := tc.ToTLSConfig()
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to parse CA certificate")
	assert.Nil(t, cfg)
}

func TestTLSConfigToTLSConfig_CertKeyFile(t *testing.T) {
	certFile := writePEMFile(t, "certkey.pem", generateCertKeyPEM(t))
	tc := &TLSConfig{Enabled: true, CertKeyFile: certFile}
	cfg, err := tc.ToTLSConfig()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Len(t, cfg.Certificates, 1)
}

func TestTLSConfigToTLSConfig_CertKeyFileError(t *testing.T) {
	badFile := writePEMFile(t, "badcert.pem", []byte("garbage"))
	tc := &TLSConfig{Enabled: true, CertKeyFile: badFile}
	cfg, err := tc.ToTLSConfig()
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to load client certificate")
	assert.Nil(t, cfg)
}

func TestDefaultTLSConfig(t *testing.T) {
	cfg := defaultTLSConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
}

func writePEMFile(t *testing.T, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, content, 0o600))
	return path
}

func generateCACertPEM(t *testing.T) []byte {
	t.Helper()
	certPEM, _ := generateCertPEM(t, "MongoDB Test CA")
	return certPEM
}

func generateCertKeyPEM(t *testing.T) []byte {
	t.Helper()
	certPEM, keyPEM := generateCertPEM(t, "MongoDB Test Client")
	return append(certPEM, keyPEM...)
}

func generateCertPEM(t *testing.T, org string) ([]byte, []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{Organization: []string{org}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return certPEM, keyPEM
}
