package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	cmd "keypub/internal/command"
	db "keypub/internal/db/.gen"
	"keypub/internal/mail"

	_ "github.com/mattn/go-sqlite3"
)

func registerCommandRegistration(registry *cmd.CommandRegistry) *cmd.CommandRegistry {

	registry.Register(cmd.Command{
		Name:        "allow",
		Usage:       "allow <email>",
		Description: `Grant permission to the given email address to see your email. The user must be registered in the system.`,
		Category:    "Privacy Control",
		Handler: func(ctx *cmd.CommandContext) (info string, err error) {
			return handleAllow(ctx.DB, ctx.Args[1], ctx.Fingerprint)
		}})
	registry.Register(cmd.Command{
		Name:        "deny",
		Usage:       "deny <email>",
		Description: `Remove permission for the given email address to see your email.`,
		Category:    "Privacy Control",
		Handler: func(ctx *cmd.CommandContext) (info string, err error) {
			return handleDeny(ctx.DB, ctx.Args[1], ctx.Fingerprint)
		},
	})
	return registry
}

func handleAllow(sqlDb *sql.DB, email, fingerprint string) (info string, err error) {
	// TODO: early exit for self allow
	// TODO: handle cases with more than 1 mail per fingerprint
	// Start transaction
	err = mail.ValidateEmail(email)
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

	// Get the email of the user with the given fingerprint
	granterEmails, err := db.New(sqlDb).WithTx(tx).
		GetUserEmailsWithFingerprint(context.TODO(), fingerprint)

	if err != nil {
		return "", fmt.Errorf("failed to query fingerprint owner: %w", err)
	}
	if len(granterEmails) == 0 {
		return "", fmt.Errorf("no user found with fingerprint: %s", fingerprint)
	}
	for _, granterEmail := range granterEmails {
		if granterEmail == email {
			return "", fmt.Errorf("you can't allow yourself, use whoami instead.")
		}
	}
	if len(granterEmails) > 1 {
		return "", fmt.Errorf("multiple users found with same fingerprint: %s", fingerprint)
	}
	granterEmail := granterEmails[0]

	// Check if the grantee exists (has any SSH keys)
	granteeCount, err := db.New(sqlDb).WithTx(tx).
		CountFingerprintWithEmail(context.TODO(), email)

	if err != nil {
		return "", fmt.Errorf("failed to query grantee existence: %w", err)
	}
	if granteeCount == 0 {
		return "", fmt.Errorf("no user found with email: %s", email)
	}

	// Check if permission already exists
	permissionCount, err := db.New(sqlDb).WithTx(tx).
		CountEmailPermissionsWithGranterAndGranteeEmail(
			context.TODO(),
			db.CountEmailPermissionsWithGranterAndGranteeEmailParams{
				GranterEmail: granterEmail,
				GranteeEmail: email,
			},
		)

	if err != nil {
		return "", fmt.Errorf("failed to query existing permissions: %w", err)
	}
	if permissionCount > 0 {
		return "permission already exists", nil
	}

	// Insert new permission
	err = db.New(sqlDb).WithTx(tx).
		AddEmailPermission(
			context.TODO(),
			db.AddEmailPermissionParams{
				GranterEmail: granterEmail,
				GranteeEmail: email,
			},
		)

	if err != nil {
		return "", fmt.Errorf("failed to insert permission: %w", err)
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	return fmt.Sprintf("Success: user %s can read your email address", email), nil
}

func handleDeny(sqlDb *sql.DB, email, fingerprint string) (info string, err error) {
	// TODO: handle cases with more than 1 mail per fingerprint
	// Start transaction
	err = mail.ValidateEmail(email)
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

	// Get the email of the user with the given fingerprint
	granterEmails, err := db.New(sqlDb).WithTx(tx).
		GetUserEmailsWithFingerprint(context.TODO(), fingerprint)

	if err != nil {
		return "", fmt.Errorf("failed to query fingerprint owner: %w", err)
	}
	if len(granterEmails) == 0 {
		return "", fmt.Errorf("no user found with fingerprint: %s", fingerprint)
	}
	for _, granterEmail := range granterEmails {
		if granterEmail == email {
			return "", fmt.Errorf("you can't deny yourself.")
		}
	}
	if len(granterEmails) > 1 {
		return "", fmt.Errorf("multiple users found with same fingerprint: %s", fingerprint)
	}
	granterEmail := granterEmails[0]

	// Delete the permission
	// TODO: sqlc by default ignores the sql result in delete query
	// which we need to check affected rows.
	err = db.New(sqlDb).WithTx(tx).
		DeleteEmailPermissionsWithGranterAndGranteeEmail(
			context.TODO(),
			db.DeleteEmailPermissionsWithGranterAndGranteeEmailParams{
				GranterEmail: granterEmail,
				GranteeEmail: email,
			},
		)

	if err != nil {
		return "", fmt.Errorf("failed to delete permission: %w", err)
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	return fmt.Sprintf("Success: user %s can no longer read your email address\n", email), nil
}
