package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	cmd "keypub/internal/command"
	db "keypub/internal/db/.gen"

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

func IsAdmin(sqlDb *sql.DB, fingerprint string) (bool, error) {
	tx, err := sqlDb.Begin()
	if err != nil {
		return false, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("failed to rollback transaction: %v", err)
		}
	}()

	count, err := db.New(sqlDb).WithTx(tx).CountAdminFingerprintswithFingerprint(context.TODO(), fingerprint)

	if err != nil {
		return false, fmt.Errorf("failed to query admin status: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return false, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return count > 0, nil
}

func AddAdmin(sqlDb *sql.DB, callerFingerprint, newAdminFingerprint string) (info string, err error) {
	tx, err := sqlDb.Begin()
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("failed to rollback transaction: %v", err)
		}
	}()

	// Check if caller is admin
	count, err := db.New(sqlDb).WithTx(tx).CountAdminFingerprintswithFingerprint(context.TODO(), callerFingerprint)
	if err != nil {
		return "", fmt.Errorf("failed to verify admin status: %w", err)
	}
	if count == 0 {
		return "", fmt.Errorf("unauthorized: only admins can add new admins")
	}

	// Add new admin
	err = db.New(sqlDb).WithTx(tx).AddAdminFingerprint(context.TODO(), newAdminFingerprint)

	if err != nil {
		return "", fmt.Errorf("failed to add admin: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	return "Admin added", nil
}

func RemoveAdmin(sqlDb *sql.DB, callerFingerprint, targetFingerprint string) (info string, err error) {
	tx, err := sqlDb.Begin()
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("failed to rollback transaction: %v", err)
		}
	}()

	// Check if caller is admin
	count, err := db.New(sqlDb).WithTx(tx).CountAdminFingerprintswithFingerprint(context.TODO(), callerFingerprint)

	if err != nil {
		return "", fmt.Errorf("failed to verify admin status: %w", err)
	}
	if count == 0 {
		return "", fmt.Errorf("unauthorized: only admins can remove admins")
	}

	count, err = db.New(sqlDb).WithTx(tx).CountAdminFingerprints(context.TODO())

	if err != nil {
		return "", fmt.Errorf("failed to count admins: %w", err)
	}
	if count <= 1 {
		return "", fmt.Errorf("cannot remove last admin")
	}

	// Remove the admin
	err = db.New(sqlDb).WithTx(tx).DeleteAdminFingerprintWithFingerprint(context.TODO(), targetFingerprint)

	if err != nil {
		return "", fmt.Errorf("failed to remove admin: %w", err)
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

func ListAdmins(sqlDb *sql.DB, callerFingerprint string) ([]AdminInfo, error) {
	tx, err := sqlDb.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("failed to rollback transaction: %v", err)
		}
	}()

	// Check if caller is admin
	count, err := db.New(sqlDb).WithTx(tx).CountAdminFingerprintswithFingerprint(context.TODO(), callerFingerprint)

	if err != nil {
		return nil, fmt.Errorf("failed to verify admin status: %w", err)
	}
	if count == 0 {
		return nil, fmt.Errorf("unauthorized: only admins can list admins")
	}

	// Get all admins
	admins, err := db.New(sqlDb).WithTx(tx).GetAdminFingerprints(context.TODO())

	if err != nil {
		return nil, fmt.Errorf("failed to list admins: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	// For backward compatibility
	var adminInfos []AdminInfo
	for _, adminInfo := range admins {
		adminInfos = append(adminInfos, AdminInfo{
			Fingerprint: adminInfo.Fingerprint,
			CreatedAt:   int32(adminInfo.CreatedAt),
		})
	}

	return adminInfos, nil
}
