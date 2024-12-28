package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	cmd "keypub/internal/command"
	"keypub/internal/db/.gen/table"

	. "github.com/go-jet/jet/v2/sqlite"
	_ "github.com/mattn/go-sqlite3"
)

func registerCommandAdmin(registry *cmd.CommandRegistry) *cmd.CommandRegistry {

	registry.Register(cmd.Command{
		Name:        "admin",
		Usage:       "admin <subcommand>",
		Description: "Administrative commands",
		Category:    "Admin",
		Subcommands: map[string]cmd.Command{
			"add": {
				Name:        "add",
				Usage:       "admin add <fingerprint>",
				Description: "add a new admin fingerprint",
				Handler: func(ctx *cmd.CommandContext) (string, error) {
					return AddAdmin(ctx.DB, ctx.Fingerprint, ctx.Args[2])
				},
			},
			"remove": {
				Name:        "remove",
				Usage:       "admin remove <fingerprint>",
				Description: "remove an admin fingerprint",
				Handler: func(ctx *cmd.CommandContext) (string, error) {
					return RemoveAdmin(ctx.DB, ctx.Fingerprint, ctx.Args[2])
				},
			},
			"list": {
				Name:        "list",
				Usage:       "admin list",
				Description: "Print fingerprint list of admins",
				Handler: func(ctx *cmd.CommandContext) (string, error) {
					admins, err := ListAdmins(ctx.DB, ctx.Fingerprint)
					if err != nil {
						return "", err
					}
					var output strings.Builder
					output.WriteString("Admin fingerprints:\n")
					for _, admin := range admins {
						output.WriteString(fmt.Sprintf("- %s\n", admin.Fingerprint))
					}
					return output.String(), nil
				},
			},
		},
	})

	registry.Register(cmd.Command{
		Name:        "shutdown",
		Usage:       "shutdown",
		Description: "Gracefully shutdown the server (admin only)",
		Category:    "Admin",
		Handler: func(ctx *cmd.CommandContext) (string, error) {
			if ctx.Server == nil {
				return "", fmt.Errorf("server shutdown not available")
			}

			isAdmin, err := IsAdmin(ctx.DB, ctx.Fingerprint)
			if err != nil {
				return "", err
			}
			if !isAdmin {
				return "", fmt.Errorf("unauthorized")
			}

			go func() {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if err := ctx.Server.Shutdown(shutdownCtx); err != nil {
					// We can't return this error since we're in a goroutine
					// You might want to log it instead
					log.Printf("Error during shutdown: %v", err)
				}
			}()

			return "Initiating graceful shutdown...", nil
		},
	})

	return registry
}

func IsAdmin(db *sql.DB, fingerprint string) (bool, error) {
	tx, err := db.Begin()
	if err != nil {
		return false, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var count []int64
	err = SELECT(COUNT(table.AdminFingerprints.Fingerprint)).
		FROM(table.AdminFingerprints).
		WHERE(table.AdminFingerprints.Fingerprint.EQ(String(fingerprint))).
		Query(tx, &count)

	if err != nil {
		return false, fmt.Errorf("failed to query admin status: %w", err)
	}
	if len(count) != 1 {
		return false, fmt.Errorf("invalid count result")
	}

	if err = tx.Commit(); err != nil {
		return false, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return count[0] > 0, nil
}

func AddAdmin(db *sql.DB, callerFingerprint, newAdminFingerprint string) (info string, err error) {
	tx, err := db.Begin()
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Check if caller is admin
	var count []int64
	err = SELECT(COUNT(table.AdminFingerprints.Fingerprint)).
		FROM(table.AdminFingerprints).
		WHERE(table.AdminFingerprints.Fingerprint.EQ(String(callerFingerprint))).
		Query(tx, &count)

	if err != nil {
		return "", fmt.Errorf("failed to verify admin status: %w", err)
	}
	if len(count) != 1 {
		return "", fmt.Errorf("invalid count result")
	}
	if count[0] == 0 {
		return "", fmt.Errorf("unauthorized: only admins can add new admins")
	}

	// Add new admin
	_, err = table.AdminFingerprints.
		INSERT(table.AdminFingerprints.Fingerprint).
		VALUES(String(newAdminFingerprint)).
		Exec(tx)

	if err != nil {
		return "", fmt.Errorf("failed to add admin: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	return "Admin added", nil
}

func RemoveAdmin(db *sql.DB, callerFingerprint, targetFingerprint string) (info string, err error) {
	tx, err := db.Begin()
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Check if caller is admin
	var count []int64
	err = SELECT(COUNT(table.AdminFingerprints.Fingerprint)).
		FROM(table.AdminFingerprints).
		WHERE(table.AdminFingerprints.Fingerprint.EQ(String(callerFingerprint))).
		Query(tx, &count)

	if err != nil {
		return "", fmt.Errorf("failed to verify admin status: %w", err)
	}
	if len(count) != 1 {
		return "", fmt.Errorf("invalid count result")
	}
	if count[0] == 0 {
		return "", fmt.Errorf("unauthorized: only admins can remove admins")
	}

	// Check total admin count
	err = SELECT(COUNT(table.AdminFingerprints.Fingerprint)).
		FROM(table.AdminFingerprints).
		Query(tx, &count)

	if err != nil {
		return "", fmt.Errorf("failed to count admins: %w", err)
	}
	if len(count) != 1 {
		return "", fmt.Errorf("invalid count result")
	}
	if count[0] <= 1 {
		return "", fmt.Errorf("cannot remove last admin")
	}

	// Remove the admin
	result, err := table.AdminFingerprints.
		DELETE().
		WHERE(table.AdminFingerprints.Fingerprint.EQ(String(targetFingerprint))).
		Exec(tx)

	if err != nil {
		return "", fmt.Errorf("failed to remove admin: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return "", fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return "", fmt.Errorf("admin not found")
	}

	if err = tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	return "Admin removed", nil
}

type AdminInfo struct {
	Fingerprint string
	CreatedAt   int32
}

func ListAdmins(db *sql.DB, callerFingerprint string) ([]AdminInfo, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Check if caller is admin
	var count []int64
	err = SELECT(COUNT(table.AdminFingerprints.Fingerprint)).
		FROM(table.AdminFingerprints).
		WHERE(table.AdminFingerprints.Fingerprint.EQ(String(callerFingerprint))).
		Query(tx, &count)

	if err != nil {
		return nil, fmt.Errorf("failed to verify admin status: %w", err)
	}
	if len(count) != 1 {
		return nil, fmt.Errorf("invalid count result")
	}
	if count[0] == 0 {
		return nil, fmt.Errorf("unauthorized: only admins can list admins")
	}

	// Get all admins
	var admins []AdminInfo
	err = SELECT(
		table.AdminFingerprints.Fingerprint.AS("admin_info.fingerprint"),
		table.AdminFingerprints.CreatedAt.AS("admin_info.created_at"),
	).FROM(
		table.AdminFingerprints,
	).ORDER_BY(
		table.AdminFingerprints.CreatedAt.ASC(),
	).Query(tx, &admins)

	if err != nil {
		return nil, fmt.Errorf("failed to list admins: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return admins, nil
}
