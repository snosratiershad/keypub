package main

import (
	"database/sql"
	"fmt"

	cmd "keypub/internal/command"
	"keypub/internal/db/.gen/table"
	"keypub/internal/mail"

	. "github.com/go-jet/jet/v2/sqlite"
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

func handleAllow(db *sql.DB, email, fingerprint string) (info string, err error) {
	// TODO: early exit for self allow
	// TODO: handle cases with more than 1 mail per fingerprint
	// Start transaction
	err = mail.ValidateEmail(email)
	if err != nil {
		return "", fmt.Errorf("mail address fails validation")
	}
	tx, err := db.Begin()
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if we don't commit

	// Get the email of the user with the given fingerprint
	var granterEmails []string
	err = SELECT(table.SSHKeys.Email).
		FROM(table.SSHKeys).
		WHERE(table.SSHKeys.Fingerprint.EQ(String(fingerprint))).
		Query(tx, &granterEmails)

	if err != nil {
		return "", fmt.Errorf("failed to query fingerprint owner: %w", err)
	}
	if len(granterEmails) == 0 {
		return "", fmt.Errorf("no user found with fingerprint: %s", fingerprint)
	}
	if len(granterEmails) > 1 {
		return "", fmt.Errorf("multiple users found with same fingerprint: %s", fingerprint)
	}
	granterEmail := granterEmails[0]

	// Check if the grantee exists (has any SSH keys)
	var granteeCount []int64
	err = SELECT(COUNT(table.SSHKeys.Email)).
		FROM(table.SSHKeys).
		WHERE(table.SSHKeys.Email.EQ(String(email))).
		Query(tx, &granteeCount)

	if err != nil {
		return "", fmt.Errorf("failed to query grantee existence: %w", err)
	}
	if len(granteeCount) != 1 {
		return "", fmt.Errorf("failed to count grantee records")
	}
	if granteeCount[0] == 0 {
		return "", fmt.Errorf("no user found with email: %s", email)
	}

	// Check if permission already exists
	var permissionCount []int64
	err = SELECT(COUNT(table.EmailPermissions.GranterEmail)).
		FROM(table.EmailPermissions).
		WHERE(
			AND(
				table.EmailPermissions.GranterEmail.EQ(String(granterEmail)),
				table.EmailPermissions.GranteeEmail.EQ(String(email)),
			),
		).
		Query(tx, &permissionCount)

	if err != nil {
		return "", fmt.Errorf("failed to query existing permissions: %w", err)
	}
	if len(permissionCount) != 1 {
		return "", fmt.Errorf("failed to count existing permissions")
	}
	if permissionCount[0] > 0 {
		return "permission already exists", nil
	}

	// Insert new permission
	_, err = table.EmailPermissions.INSERT(
		table.EmailPermissions.GranterEmail,
		table.EmailPermissions.GranteeEmail,
	).VALUES(
		granterEmail,
		email,
	).Exec(tx)

	if err != nil {
		return "", fmt.Errorf("failed to insert permission: %w", err)
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	return fmt.Sprintf("Success: user %s can read your email address", email), nil
}

func handleDeny(db *sql.DB, email, fingerprint string) (info string, err error) {
	// TODO: handle cases with more than 1 mail per fingerprint
	// Start transaction
	err = mail.ValidateEmail(email)
	if err != nil {
		return "", fmt.Errorf("mail address fails validation")
	}
	tx, err := db.Begin()
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if we don't commit

	// Get the email of the user with the given fingerprint
	var granterEmails []string
	err = SELECT(table.SSHKeys.Email).
		FROM(table.SSHKeys).
		WHERE(table.SSHKeys.Fingerprint.EQ(String(fingerprint))).
		Query(tx, &granterEmails)

	if err != nil {
		return "", fmt.Errorf("failed to query fingerprint owner: %w", err)
	}
	if len(granterEmails) == 0 {
		return "", fmt.Errorf("no user found with fingerprint: %s", fingerprint)
	}
	if len(granterEmails) > 1 {
		return "", fmt.Errorf("multiple users found with same fingerprint: %s", fingerprint)
	}
	granterEmail := granterEmails[0]

	// Delete the permission
	result, err := table.EmailPermissions.DELETE().
		WHERE(
			AND(
				table.EmailPermissions.GranterEmail.EQ(String(granterEmail)),
				table.EmailPermissions.GranteeEmail.EQ(String(email)),
			),
		).
		Exec(tx)

	if err != nil {
		return "", fmt.Errorf("failed to delete permission: %w", err)
	}

	// Check if any rows were affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return "", fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return "", fmt.Errorf("no permission found for email: %s", email)
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	return fmt.Sprintf("Success: user %s can no longer read your email address\n", email), nil
}
