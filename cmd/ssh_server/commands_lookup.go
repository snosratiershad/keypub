package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	cmd "keypub/internal/command"
	db "keypub/internal/db/.gen"

	_ "github.com/mattn/go-sqlite3"
)

func registerCommandLookup(registry *cmd.CommandRegistry) *cmd.CommandRegistry {

	registry.Register(cmd.Command{
		Name:        "get",
		Usage:       "get <subcommand>",
		Description: "Get information about users",
		Category:    "Information",
		Subcommands: map[string]cmd.Command{
			"email": {
				Name:        "email",
				Usage:       "get email <fingerprint>",
				Description: "Get email for the given fingerprint (if authorized)",
				Handler: func(ctx *cmd.CommandContext) (string, error) {
					targetFingerprint := ctx.Args[2]
					return handleGetEmail(ctx.DB, ctx.Fingerprint, targetFingerprint)
				},
			},
		},
	})
	return registry
}

func handleGetEmail(sqlDb *sql.DB, callerFingerprint, targetFingerprint string) (string, error) {
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

	// First get the caller's email
	callerEmails, err := db.New(sqlDb).WithTx(tx).
		GetUserEmailsWithFingerprint(context.TODO(), callerFingerprint)

	if err != nil {
		return "", fmt.Errorf("failed to query caller info: %w", err)
	}
	if len(callerEmails) == 0 {
		return "", fmt.Errorf("caller not registered")
	}
	if len(callerEmails) > 1 {
		return "", fmt.Errorf("multiple emails found for caller fingerprint")
	}
	callerEmail := callerEmails[0]

	targetInfo, err := db.New(sqlDb).WithTx(tx).
		GetTargetInfoWithFingerprintAndGranterEmailAndGranteeEmail(
			context.TODO(),
			db.GetTargetInfoWithFingerprintAndGranterEmailAndGranteeEmailParams{
				GranteeEmail: callerEmail,
				Email:        callerEmail,
				Fingerprint:  targetFingerprint,
			},
		)

	if err != nil {
		return "", fmt.Errorf("failed to query target info: %w", err)
	}
	if len(targetInfo) == 0 {
		return "", fmt.Errorf("no email found or permission denied")
	}
	if len(targetInfo) > 1 {
		return "", fmt.Errorf("multiple emails found for target fingerprint")
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	return targetInfo[0], nil
}
