package main

import (
	"fmt"
	"io"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

func handleRegister(s ssh.Session, email string) {
	fingerprint := gossh.FingerprintSHA256(s.PublicKey())
	io.WriteString(s, fmt.Sprintf("Processing registration for %s with key fingerprint %s\n", email, fingerprint))
	// TODO: Implement actual registration logic
}

func handleConfirm(s ssh.Session, code string) {
	io.WriteString(s, fmt.Sprintf("Processing confirmation code: %s\n", code))
	// TODO: Implement confirmation logic
}

func handleAdd(s ssh.Session, fieldValue string) {
	io.WriteString(s, fmt.Sprintf("Adding field:value pair: %s\n", fieldValue))
	// TODO: Implement add field logic
}

func handleGet(s ssh.Session, field string, keyData string) {
	io.WriteString(s, fmt.Sprintf("Getting %s from key %s\n", field, keyData))
	// TODO: Implement get field logic
}
