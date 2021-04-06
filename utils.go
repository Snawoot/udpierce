package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io/ioutil"
	"net"
	"time"
)

const DGRAM_BUF = int(^uint16(0)) + 1
const DGRAM_LEN_BYTES = 2
const RESOLVE_ATTEMPTS = 3

func makeServerTLSConfig(certfile, keyfile, cafile string) (*tls.Config, error) {
	var cfg tls.Config
	cert, err := tls.LoadX509KeyPair(certfile, keyfile)
	if err != nil {
		return nil, err
	}
	cfg.Certificates = []tls.Certificate{cert}
	if cafile != "" {
		roots := x509.NewCertPool()
		certs, err := ioutil.ReadFile(cafile)
		if err != nil {
			return nil, err
		}
		if ok := roots.AppendCertsFromPEM(certs); !ok {
			return nil, errors.New("Failed to load CA certificates")
		}
		cfg.ClientCAs = roots
		cfg.ClientAuth = tls.VerifyClientCertIfGiven
	}
	return &cfg, nil
}

func makeClientTLSConfig(servername, certfile, keyfile, cafile string,
	hostname_check bool) (*tls.Config, error) {
	if !hostname_check && cafile == "" {
		return nil, errors.New("Hostname check should not be disabled in absence of custom CA file")
	}
	if certfile != "" && keyfile == "" || certfile == "" && keyfile != "" {
		return nil, errors.New("Certificate file and key file must be specified only together")
	}
	var certs []tls.Certificate
	if certfile != "" && keyfile != "" {
		cert, err := tls.LoadX509KeyPair(certfile, keyfile)
		if err != nil {
			return nil, err
		}
		certs = append(certs, cert)
	}
	var roots *x509.CertPool
	if cafile != "" {
		roots = x509.NewCertPool()
		certs, err := ioutil.ReadFile(cafile)
		if err != nil {
			return nil, err
		}
		if ok := roots.AppendCertsFromPEM(certs); !ok {
			return nil, errors.New("Failed to load CA certificates")
		}
	}
	tlsConfig := tls.Config{
		RootCAs:      roots,
		ServerName:   servername,
		Certificates: certs,
	}
	if !hostname_check {
		tlsConfig.InsecureSkipVerify = true
		tlsConfig.VerifyPeerCertificate = func(certificates [][]byte, _ [][]*x509.Certificate) error {
			certs := make([]*x509.Certificate, len(certificates))
			for i, asn1Data := range certificates {
				cert, err := x509.ParseCertificate(asn1Data)
				if err != nil {
					return errors.New("tls: failed to parse certificate from server: " + err.Error())
				}
				certs[i] = cert
			}

			opts := x509.VerifyOptions{
				Roots:         roots, // On the server side, use config.ClientCAs.
				DNSName:       "",    // No hostname check
				Intermediates: x509.NewCertPool(),
			}
			for _, cert := range certs[1:] {
				opts.Intermediates.AddCert(cert)
			}
			_, err := certs[0].Verify(opts)
			return err
		}
	}
	return &tlsConfig, nil
}

func ProbeResolveTCP(address string, timeout time.Duration) (string, error) {
	var (
		conn net.Conn
		err  error
	)
	for i := 0; i < RESOLVE_ATTEMPTS; i++ {
		conn, err = net.DialTimeout("tcp", address, timeout)
		if err == nil {
			resolved := conn.RemoteAddr().String()
			conn.Close()
			return resolved, nil
		}
	}
	return "", err
}
