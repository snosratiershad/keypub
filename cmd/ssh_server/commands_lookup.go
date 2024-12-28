package main

import (
	"database/sql"
	"fmt"

	. "github.com/go-jet/jet/v2/sqlite"
	_ "github.com/mattn/go-sqlite3"
	cmd "keypub/internal/command"
	"keypub/internal/db/.gen/table"
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

func handleGetEmail(db *sql.DB, callerFingerprint, targetFingerprint string) (string, error) {
	// TODO: handle cases with more than 1 mail per fingerprint
	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if we don't commit

	// First get the caller's email
	var callerEmails []string
	err = SELECT(table.SSHKeys.Email).
		FROM(table.SSHKeys).
		WHERE(table.SSHKeys.Fingerprint.EQ(String(callerFingerprint))).
		Query(tx, &callerEmails)

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

	// Get target's email and check permissions
	type TargetInfo struct {
		Email string
	}
	var targetInfo []TargetInfo
	err = SELECT(
		table.SSHKeys.Email.AS("target_info.email"),
	).FROM(
		table.SSHKeys.
			LEFT_JOIN(table.EmailPermissions, AND(
				table.EmailPermissions.GranterEmail.EQ(table.SSHKeys.Email),
				table.EmailPermissions.GranteeEmail.EQ(String(callerEmail)),
			)),
	).WHERE(
		AND(
			table.SSHKeys.Fingerprint.EQ(String(targetFingerprint)),
			OR(
				// Either the caller is looking up their own email
				table.SSHKeys.Email.EQ(String(callerEmail)),
				// Or they have permission
				table.EmailPermissions.GranteeEmail.IS_NOT_NULL(),
			),
		),
	).Query(tx, &targetInfo)

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

	return targetInfo[0].Email, nil
}
