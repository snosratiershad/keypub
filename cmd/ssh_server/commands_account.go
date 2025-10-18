package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	cmd "keypub/internal/command"
	db "keypub/internal/db/.gen"
	"keypub/internal/mail"

	_ "github.com/mattn/go-sqlite3"
)

func registerCommandAccount(registry *cmd.CommandRegistry) *cmd.CommandRegistry {

	registry.Register(cmd.Command{
		Name:        "whoami",
		Usage:       "whoami",
		Description: "Show your fingerprint, registered email, registration date, and list of users allowed to see your email.",
		Category:    "Account",
		Handler: func(ctx *cmd.CommandContext) (info string, err error) {
			return handleWhoami(ctx.DB, ctx.Fingerprint)
		},
	})

	registry.Register(cmd.Command{
		Name:        "register",
		Usage:       "register <email>",
		Description: "Register your SSH key with the given email address. You will receive a confirmation code via email.",
		Category:    "Account",
		Handler: func(ctx *cmd.CommandContext) (info string, err error) {
			return handleRegister(ctx.DB, ctx.MailSender, ctx.Args[1], ctx.Fingerprint)
		},
	})
	registry.Register(cmd.Command{
		Name:        "confirm",
		Usage:       "confirm <code>",
		Description: "Confirm your email address using the code you received. This completes your registration.",
		Category:    "Account",
		Handler: func(ctx *cmd.CommandContext) (info string, err error) {
			return handleConfirm(ctx.DB, ctx.Fingerprint, ctx.Args[1])
		},
	})
	registry.Register(cmd.Command{
		Name:        "unregister",
		Usage:       "unregister",
		Description: "Remove your registration and all associated permissions. This cannot be undone.",
		Category:    "Account",
		Handler: func(ctx *cmd.CommandContext) (info string, err error) {
			return handleUnregister(ctx.DB, ctx.Fingerprint)
		},
	})
	return registry
}

func handleWhoami(sqlDb *sql.DB, fingerprint string) (string, error) {
	// TODO: handle cases with more than 1 mail per fingerprint
	// Start transaction
	tx, err := sqlDb.Begin()
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("failed to rollback transaction: %v", err)
		}
	}()

	// First get the email for the current fingerprint
	userEmails, err := db.New(sqlDb).WithTx(tx).
		GetUserEmailsWithFingerprint(
			context.TODO(),
			fingerprint,
		)
	if err != nil {
		return "", fmt.Errorf("failed to query user email: %w", err)
	}
	if len(userEmails) == 0 {
		return fmt.Sprintf("You are not registered. Your fingerprint is %s", fingerprint), nil
	}
	if len(userEmails) > 1 {
		return "", fmt.Errorf("multiple emails found for fingerprint: %s", fingerprint)
	}
	userEmail := userEmails[0]

	keyInfos, err := db.New(sqlDb).WithTx(tx).
		GetKeyInfosWithEmail(context.TODO(), userEmail)

	if err != nil {
		return "", fmt.Errorf("failed to query key info: %w", err)
	}

	allowedUsers, err := db.New(sqlDb).WithTx(tx).
		GetAllowedUsersWithGranterEmail(context.TODO(), userEmail)

	if err != nil {
		return "", fmt.Errorf("failed to query allowed users: %w", err)
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Format the output
	var result strings.Builder

	// Format user info
	result.WriteString(fmt.Sprintf("Email: %s\n\n", userEmail))
	result.WriteString("Registered Keys:\n")

	for _, key := range keyInfos {
		createdTime := time.Unix(int64(key.CreatedAt), 0)
		if key.Fingerprint == fingerprint {
			result.WriteString(fmt.Sprintf("* %s (current) - registered: %s\n",
				key.Fingerprint,
				createdTime.Format(time.RFC3339)))
		} else {
			result.WriteString(fmt.Sprintf("  %s - registered: %s\n",
				key.Fingerprint,
				createdTime.Format(time.RFC3339)))
		}
	}

	// Format allowed users
	if len(allowedUsers) == 0 {
		result.WriteString("\nNo users are allowed to see your email.")
	} else {
		result.WriteString("\nAllowed users:\n")
		for _, user := range allowedUsers {
			grantTime := time.Unix(int64(user.CreatedAt), 0)
			result.WriteString(fmt.Sprintf("- %s (granted: %s)\n",
				user.GranteeEmail,
				grantTime.Format(time.RFC3339)))
		}
	}

	return result.String(), nil
}

func generateVerificationCode() string {
	// TODO: this is not really uniform random
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const length = 6

	// Create a byte slice to store the result
	result := make([]byte, length)

	// Use crypto/rand for secure random number generation
	for i := range result {
		// Read a random byte and map it to the charset
		b := make([]byte, 1)
		_, _ = rand.Read(b)
		result[i] = charset[b[0]%byte(len(charset))]
	}

	return string(result)
}

func handleRegister(sqlDb *sql.DB, mail_sender mail.MailSender, to_email string, fingerprint string) (info string, err error) {
	// TODO: allow more than 1 mail per fingerprint
	// Start transaction
	err = mail.ValidateEmail(to_email)
	if err != nil {
		return "", fmt.Errorf("mail address fails validation")
	}
	tx, err := sqlDb.Begin()
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("failed to rollback transaction: %v", err)
		}
	}()

	count, err := db.New(sqlDb).WithTx(tx).
		CountFingerprintWithEmailAndFingerPrint(
			context.TODO(),
			db.CountFingerprintWithEmailAndFingerPrintParams{
				Email:       to_email,
				Fingerprint: fingerprint,
			},
		)
	if err != nil {
		return "", fmt.Errorf("failed to query existing keys: %w", err)
	}

	if count > 0 {
		return "", fmt.Errorf("email and fingerprint combination already registered")
	}

	// Check if email and fingerprint combination exists using COUNT
	count, err = db.New(sqlDb).WithTx(tx).CountVerificationCodeWithFingerprint(context.TODO(), fingerprint)

	if err != nil {
		return "", fmt.Errorf("failed to query existing keys in verification codes table: %w", err)
	}

	if count > 0 {
		return "", fmt.Errorf("Verification mail has already been sent. It will expire within 1hr")
	}

	///////////////////////////
	// Generate verification code
	verificationCode := generateVerificationCode() // You'll need to implement this

	_, err = db.New(sqlDb).WithTx(tx).AddVerificationCode(
		context.TODO(),
		db.AddVerificationCodeParams{
			Email:       to_email,
			Fingerprint: fingerprint,
			Code:        verificationCode,
		},
	)

	if err != nil {
		return "", fmt.Errorf("failed to insert verification code: %w", err)
	}

	ctx := context.Background()
	err = mail_sender.SendConfirmation(ctx, to_email, verificationCode, fingerprint)
	if err != nil {
		return "", fmt.Errorf("Could not send confirmation mail: %s", err)
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	return "Success: Confirmation mail sent", nil
}

func handleConfirm(sqlDb *sql.DB, fingerprint string, code string) (info string, err error) {
	// TODO: allow for multiple mails per fingerprint
	// Start transaction
	tx, err := sqlDb.Begin()
	if err != nil {
		return "", err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("failed to rollback transaction: %v", err)
		}
	}()

	// Get the verification record
	emails, err := db.New(sqlDb).WithTx(tx).
		GetVerificationCodeEmailsWithFingerprintAndCode(
			context.TODO(),
			db.GetVerificationCodeEmailsWithFingerprintAndCodeParams{
				Fingerprint: fingerprint,
				Code:        code,
			},
		)

	if err != nil {
		return "", fmt.Errorf("could not find verification request for fingerprint and code: %s", err)
	}

	if len(emails) > 1 {
		return "", fmt.Errorf("too many matching verifications found: %d", len(emails))
	}

	if len(emails) == 0 {
		return "", fmt.Errorf("could not find verification request for fingerprint and code")
	}

	email := emails[0]

	// Delete the verification record
	err = db.New(sqlDb).WithTx(tx).DeleteVerificationCodeWithFingerprintAndCode(
		context.TODO(),
		db.DeleteVerificationCodeWithFingerprintAndCodeParams{
			Fingerprint: fingerprint,
			Code:        code,
		},
	)

	if err != nil {
		return "", fmt.Errorf("could not delete verification: %s", err)
	}

	// Create the SSH key entry
	_, err = db.New(sqlDb).WithTx(tx).AddSSHKey(
		context.TODO(),
		db.AddSSHKeyParams{
			Fingerprint: fingerprint,
			Email:       email,
		},
	)

	if err != nil {
		return "", fmt.Errorf("failed to register: %w", err)
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		return "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	return fmt.Sprintf("Success: email %s is now associated with fingerprint %s", email, fingerprint), nil
}

func handleUnregister(sqlDb *sql.DB, fingerprint string) (info string, err error) {
	// TODO: handle cases with more than 1 mail per fingerprint
	// Start transaction
	tx, err := sqlDb.Begin()
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("failed to rollback transaction: %v", err)
		}
	}()

	// First get the user's email
	emails, err := db.New(sqlDb).WithTx(tx).GetUserEmailsWithFingerprint(context.TODO(), fingerprint)

	if err != nil {
		return "", fmt.Errorf("failed to query user email: %w", err)
	}
	if len(emails) == 0 {
		return "", fmt.Errorf("no registration found for this fingerprint")
	}
	if len(emails) > 1 {
		return "", fmt.Errorf("multiple registrations found for this fingerprint")
	}
	email := emails[0]

	// Count remaining keys for this email
	keyCount, err := db.New(sqlDb).WithTx(tx).CountFingerprintWithEmail(context.TODO(), email)

	if err != nil {
		return "", fmt.Errorf("failed to count remaining keys: %w", err)
	}

	// Only delete permissions if this is the last key
	if keyCount == 1 {
		// Delete all permissions where this user is the granter
		err = db.New(sqlDb).WithTx(tx).DeletePermissionsWithGranterEmail(context.TODO(), email)

		if err != nil {
			return "", fmt.Errorf("failed to delete granted permissions: %w", err)
		}

		// Delete all permissions where this user is the grantee
		err = db.New(sqlDb).WithTx(tx).DeletePermissionsWithGranteeEmail(context.TODO(), email)
		if err != nil {
			return "", fmt.Errorf("failed to delete received permissions: %w", err)
		}
	}

	// Delete any pending verification codes for this fingerprint
	err = db.New(sqlDb).WithTx(tx).DeleteVerificationCodesWithFingerprint(context.TODO(), fingerprint)

	if err != nil {
		return "", fmt.Errorf("failed to delete verification codes: %w", err)
	}
	// Delete admin status for this user, if exists
	err = db.New(sqlDb).WithTx(tx).DeleteAdminFingerprintWithFingerprint(context.TODO(), fingerprint)

	if err != nil {
		return "", fmt.Errorf("failed to delete admin status: %w", err)
	}

	// Delete the specific SSH key registration
	err = db.New(sqlDb).WithTx(tx).DeleteSSHKeysWithFingerprint(context.TODO(), fingerprint)

	// Verify that we actually deleted a registration
	if err != nil {
		return "", fmt.Errorf("failed to get rows affected: %w", err)
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	return "Success: Your registration and all related permissions have been removed", nil
}
