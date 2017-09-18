package netutils

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
)

// GetTLSConfigFromCerts construct tls.tlsConfig from files
func GetTLSConfigFromCerts(cert, key, cacert string) (*tls.Config, error) {
	tlsCert, _ := tls.LoadX509KeyPair(cert, key)

	caCert, err := ioutil.ReadFile(cacert)
	if err != nil {
		return nil, fmt.Errorf("error load certs: %s", err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		RootCAs:      caCertPool,
	}, nil
}
