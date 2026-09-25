package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

func BuildTLSCAConfig(trustedCAFiles []string) (*tls.Config, error) {
	rootCAs, err := newCertPool(trustedCAFiles)
	if err != nil {
		return nil, fmt.Errorf("no valid certificates found in %v", trustedCAFiles)
	}

	return &tls.Config{
		MinVersion: tls.VersionTLS13,
		RootCAs:    rootCAs,
	}, nil
}

func BuildTLSConfig(certFile, keyFile string, trustedCAFiles []string) (*tls.Config, error) {
	config, err := BuildTLSCAConfig(trustedCAFiles)
	if err != nil {
		return nil, err
	}

	config.GetCertificate = func(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		return newCert(certFile, keyFile)
	}
	config.GetClientCertificate = func(unused *tls.CertificateRequestInfo) (*tls.Certificate, error) {
		return newCert(certFile, keyFile)
	}
	return config, nil
}

func newCertPool(trustedCAFiles []string) (*x509.CertPool, error) {
	certPool, err := x509.SystemCertPool()
	if err != nil {
		certPool = x509.NewCertPool()
	}
	for _, f := range trustedCAFiles {
		pemByte, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}

		for {
			var block *pem.Block
			block, pemByte = pem.Decode(pemByte)
			if block == nil {
				break
			}
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, err
			}
			certPool.AddCert(cert)
		}
	}

	return certPool, nil
}

func newCert(certFile, keyFile string) (*tls.Certificate, error) {
	cert, err := os.ReadFile(certFile)
	if err != nil {
		return nil, err
	}

	key, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, err
	}

	tlsCert, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}
	return &tlsCert, nil
}