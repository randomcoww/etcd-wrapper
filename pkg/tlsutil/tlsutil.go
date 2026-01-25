package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"os"
)

func TLSCAConfig(trustedCAFiles []string) (*tls.Config, error) {
	rootCAs, err := newCertPool(trustedCAFiles)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		MinVersion: tls.VersionTLS13,
		RootCAs:    rootCAs,
	}, nil
}

func TLSConfig(trustedCAFiles []string, clientCertFile, clientKeyFile string) (*tls.Config, error) {
	config, err := TLSCAConfig(trustedCAFiles)
	if err != nil {
		return nil, err
	}

	config.GetCertificate = func(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		return newCert(clientCertFile, clientKeyFile)
	}
	config.GetClientCertificate = func(unused *tls.CertificateRequestInfo) (*tls.Certificate, error) {
		return newCert(clientCertFile, clientKeyFile)
	}
	return config, nil
}

func newCertPool(CAFiles []string) (*x509.CertPool, error) {
	certPool, err := x509.SystemCertPool()
	if err != nil {
		certPool = x509.NewCertPool()
	}
	for _, CAFile := range CAFiles {
		pemByte, err := os.ReadFile(CAFile)
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

func newCert(certfile, keyfile string) (*tls.Certificate, error) {
	cert, err := os.ReadFile(certfile)
	if err != nil {
		return nil, err
	}

	key, err := os.ReadFile(keyfile)
	if err != nil {
		return nil, err
	}

	tlsCert, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}
	return &tlsCert, nil
}
