package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"time"
)

func getTLSConfig() (*tls.Config, error) {
	certPath := "certs/server.crt"
	keyPath := "certs/server.key"

	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			cert, err := tls.LoadX509KeyPair(certPath, keyPath)
			if err != nil {
				return nil, fmt.Errorf("failed to load existing cert: %w", err)
			}
			log.Printf("[TLS] Loaded existing certificate")
			return &tls.Config{
				Certificates: []tls.Certificate{cert},
				MinVersion:   tls.VersionTLS12,
			}, nil
		}
	}

	log.Printf("[TLS] Generating self-signed certificate...")
	if err := generateSelfSignedCert(certPath, keyPath); err != nil {
		return nil, fmt.Errorf("failed to generate cert: %w", err)
	}

	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load generated cert: %w", err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

func generateSelfSignedCert(certPath, keyPath string) error {
	os.MkdirAll("certs", 0700)

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		return err
	}

	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "47.110.248.240",
			Organization: []string{"LingQiao"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(3650 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:           []net.IP{net.ParseIP("47.110.248.240"), net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost"},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return err
	}

	keyPEM, err := os.Create(keyPath)
	if err != nil {
		return err
	}
	defer keyPEM.Close()
	pem.Encode(keyPEM, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	certPEM, err := os.Create(certPath)
	if err != nil {
		return err
	}
	defer certPEM.Close()
	pem.Encode(certPEM, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	return nil
}
