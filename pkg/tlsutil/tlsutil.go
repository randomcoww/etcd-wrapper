package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
)

func TLSConfig(trustedCAFile, clientCertFile, clientKeyFile string) (*tls.Config, error) {
	rootCAs, err := newCertPool([]string{trustedCAFile})
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		MinVersion: tls.VersionTLS13,
		RootCAs:    rootCAs,
		GetCertificate: func(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			return newCert(clientCertFile, clientKeyFile)
		},
		GetClientCertificate: func(unused *tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return newCert(clientCertFile, clientKeyFile)
		},
	}, nil
}

func newCertPool(CAFiles []string) (*x509.CertPool, error) {
	certPool, err := x509.SystemCertPool()
	if err != nil {
		certPool = x509.NewCertPool()
	}
	for _, CAFile := range CAFiles {
		pemByte, err := ioutil.ReadFile(CAFile)
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
	cert, err := ioutil.ReadFile(certfile)
	if err != nil {
		return nil, err
	}

	key, err := ioutil.ReadFile(keyfile)
	if err != nil {
		return nil, err
	}

	tlsCert, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}
	return &tlsCert, nil
}
