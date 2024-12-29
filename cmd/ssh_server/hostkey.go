package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"os"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

func loadHostKey(path, passphrase string) (ssh.Signer, error) {
	keyBytes, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("No existing host key found, generating new one...")
			key, err := rsa.GenerateKey(rand.Reader, 4096)
			if err != nil {
				return nil, fmt.Errorf("failed to generate host key: %w", err)
			}

			// Save the private key in PEM format
			privPEM := &pem.Block{
				Type:  "RSA PRIVATE KEY",
				Bytes: x509.MarshalPKCS1PrivateKey(key),
			}
			keyBytes := pem.EncodeToMemory(privPEM)

			err = os.WriteFile(path, keyBytes, 0600)
			if err != nil {
				return nil, fmt.Errorf("failed to save host key: %w", err)
			}

			signer, err := gossh.NewSignerFromKey(key)
			if err != nil {
				return nil, fmt.Errorf("failed to create signer: %w", err)
			}

			return signer, nil
		}
		return nil, fmt.Errorf("failed to load host key: %w", err)
	}

	log.Printf("Loading existing host key...")
	var signer ssh.Signer
	if passphrase == "" {
		signer, err = gossh.ParsePrivateKey(keyBytes)
	} else {
		signer, err = gossh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(passphrase))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to parse host key: %w", err)
	}

	return signer, nil
}
