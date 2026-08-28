package mongodb

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
)

const minTLSVersion = tls.VersionTLS12

type TLSConfig struct {
	Enabled            bool   `mapstructure:"enabled"`
	CAFile             string `mapstructure:"ca_file"`
	CertKeyFile        string `mapstructure:"cert_key_file"`
	InsecureSkipVerify bool   `mapstructure:"insecure_skip_verify"`
}

func (t *TLSConfig) ToTLSConfig() (*tls.Config, error) {
	if t == nil || !t.Enabled {
		return nil, nil
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: t.InsecureSkipVerify, // #nosec G402 -- TLS verification is intentionally configurable
	}

	if t.CAFile != "" {
		caCert, err := os.ReadFile(t.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, errors.New("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
	}

	if t.CertKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(t.CertKeyFile, t.CertKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}

func defaultTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion: minTLSVersion,
	}
}
