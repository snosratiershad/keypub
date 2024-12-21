package main

import (
	"crypto/rand"
	"fmt"
	"io"
	"time"

	"database/sql"
	"keypub/internal/db/.gen/table"
	"keypub/internal/mail"

	"github.com/gliderlabs/ssh"
	. "github.com/go-jet/jet/v2/sqlite"
)

func generateVerificationCode() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const length = 6

	// Create a byte slice to store the result
	result := make([]byte, length)

	// Use crypto/rand for secure random number generation
	for i := range result {
		// Read a random byte and map it to the charset
		b := make([]byte, 1)
		rand.Read(b)
		result[i] = charset[b[0]%byte(len(charset))]
	}

	return string(result)
}

func handleRegister(db *sql.DB, mail_sender *mail.MailSender, to_email string, fingerprint string) error {
	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if we don't commit

	// Check if email and fingerprint combination exists using COUNT
	stmt := SELECT(
		COUNT(table.SSHKeys.Fingerprint),
	).FROM(
		table.SSHKeys,
	).WHERE(
		AND(
			table.SSHKeys.Email.EQ(String(to_email)),
			table.SSHKeys.Fingerprint.EQ(String(fingerprint)),
		),
	)

	var counts []int64
	err = stmt.Query(tx, &counts)
	if err != nil {
		return fmt.Errorf("failed to query existing keys: %w", err)
	}
	if len(counts) != 1 {
		return fmt.Errorf("could not count email and fingerprint pairs in db. len(count)=%d", len(counts))
	}

	count := counts[0]
	if count > 0 {
		return fmt.Errorf("email and fingerprint combination already exists")
	}

	// Generate verification code
	verificationCode := generateVerificationCode() // You'll need to implement this

	now := time.Now()
	// Insert verification code
	insertStmt := table.VerificationCodes.INSERT(
		table.VerificationCodes.Email,
		table.VerificationCodes.Fingerprint,
		table.VerificationCodes.Code,
		table.VerificationCodes.CreatedAt,
	).VALUES(
		to_email,
		fingerprint,
		verificationCode,
		now,
	).ON_CONFLICT(
		table.VerificationCodes.Email,
		table.VerificationCodes.Fingerprint,
	).DO_UPDATE(
		SET(
			table.VerificationCodes.Code.SET(String(verificationCode)),
			table.VerificationCodes.CreatedAt.SET(Int64(now.Unix())),
		),
	)

	_, err = insertStmt.Exec(tx)
	if err != nil {
		return fmt.Errorf("failed to insert verification code: %w", err)
	}

	err = mail_sender.SendConfirmation(to_email, verificationCode, fingerprint)
	if err != nil {
		return fmt.Errorf("Could not send confirmation mail: %s", err)
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func handleConfirm(db *sql.DB, fingerprint string, code string) (error, string) {
	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		return err, ""
	}
	defer tx.Rollback() // Will be ignored if transaction is committed

	// Get the verification record
	var emails []string
	err = SELECT(table.VerificationCodes.Email).
		FROM(table.VerificationCodes).
		WHERE(
			AND(
				table.VerificationCodes.Fingerprint.EQ(String(fingerprint)),
				table.VerificationCodes.Code.EQ(String(code)),
			),
		).
		Query(tx, &emails)

	if err != nil {
		return fmt.Errorf("could not find verification request for fingerprint and code: %s", err), ""
	}

	if len(emails) > 1 {
		return fmt.Errorf("too many matching verifications found: %d", len(emails)), ""
	}

	if len(emails) == 0 {
		return fmt.Errorf("could not find verification request for fingerprint and code"), ""
	}

	email := emails[0]

	// Delete the verification record
	_, err = table.VerificationCodes.DELETE().
		WHERE(
			AND(
				table.VerificationCodes.Fingerprint.EQ(String(fingerprint)),
				table.VerificationCodes.Code.EQ(String(code)),
			),
		).
		Exec(tx)

	if err != nil {
		return fmt.Errorf("could not delete verification: %s", err), ""
	}

	// Create the SSH key entry
	_, err = table.SSHKeys.INSERT(
		table.SSHKeys.Fingerprint,
		table.SSHKeys.Email,
	).
		VALUES(
			fingerprint,
			email,
		).
		Exec(tx)

	if err != nil {
		return err, ""
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		return err, ""
	}

	return nil, email
}

func handleAdd(s ssh.Session, fieldValue string) {
	io.WriteString(s, fmt.Sprintf("Adding field:value pair: %s\n", fieldValue))
	// TODO: Implement add field logic
}

func handleGet(s ssh.Session, field string, keyData string) {
	io.WriteString(s, fmt.Sprintf("Getting %s from key %s\n", field, keyData))
	// TODO: Implement get field logic
}
