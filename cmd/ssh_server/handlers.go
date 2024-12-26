package main

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"database/sql"
	"keypub/internal/db/.gen/table"
	"keypub/internal/mail"

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
		_, _ = rand.Read(b)
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
		return fmt.Errorf("email and fingerprint combination already registered")
	}

	////////////////////////////
	// Check if email and fingerprint combination exists using COUNT
	stmt = SELECT(
		COUNT(table.VerificationCodes.Fingerprint),
	).FROM(
		table.VerificationCodes,
	).WHERE(
		table.VerificationCodes.Fingerprint.EQ(String(fingerprint)),
	)

	counts = nil
	err = stmt.Query(tx, &counts)
	if err != nil {
		return fmt.Errorf("failed to query existing keys in verification codes table: %w", err)
	}
	if len(counts) != 1 {
		return fmt.Errorf("could not count email and fingerprint pairs in db (verification codes table). len(count)=%d", len(counts))
	}

	count = counts[0]
	if count > 0 {
		return fmt.Errorf("Verification mail has already been sent. It will expire within 1hr")
	}

	///////////////////////////
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
		return fmt.Errorf("failed to register: %w", err), ""
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err), ""
	}

	return nil, email
}

func handleAllow(db *sql.DB, email, fingerprint string) error {
	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if we don't commit

	// Get the email of the user with the given fingerprint
	var granterEmails []string
	err = SELECT(table.SSHKeys.Email).
		FROM(table.SSHKeys).
		WHERE(table.SSHKeys.Fingerprint.EQ(String(fingerprint))).
		Query(tx, &granterEmails)

	if err != nil {
		return fmt.Errorf("failed to query fingerprint owner: %w", err)
	}
	if len(granterEmails) == 0 {
		return fmt.Errorf("no user found with fingerprint: %s", fingerprint)
	}
	if len(granterEmails) > 1 {
		return fmt.Errorf("multiple users found with same fingerprint: %s", fingerprint)
	}
	granterEmail := granterEmails[0]

	// Check if the grantee exists (has any SSH keys)
	var granteeCount []int64
	err = SELECT(COUNT(table.SSHKeys.Email)).
		FROM(table.SSHKeys).
		WHERE(table.SSHKeys.Email.EQ(String(email))).
		Query(tx, &granteeCount)

	if err != nil {
		return fmt.Errorf("failed to query grantee existence: %w", err)
	}
	if len(granteeCount) != 1 {
		return fmt.Errorf("failed to count grantee records")
	}
	if granteeCount[0] == 0 {
		return fmt.Errorf("no user found with email: %s", email)
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
		return fmt.Errorf("failed to query existing permissions: %w", err)
	}
	if len(permissionCount) != 1 {
		return fmt.Errorf("failed to count existing permissions")
	}
	if permissionCount[0] > 0 {
		return nil // Permission already exists, return without error
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
		return fmt.Errorf("failed to insert permission: %w", err)
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func handleDeny(db *sql.DB, email, fingerprint string) error {
	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if we don't commit

	// Get the email of the user with the given fingerprint
	var granterEmails []string
	err = SELECT(table.SSHKeys.Email).
		FROM(table.SSHKeys).
		WHERE(table.SSHKeys.Fingerprint.EQ(String(fingerprint))).
		Query(tx, &granterEmails)

	if err != nil {
		return fmt.Errorf("failed to query fingerprint owner: %w", err)
	}
	if len(granterEmails) == 0 {
		return fmt.Errorf("no user found with fingerprint: %s", fingerprint)
	}
	if len(granterEmails) > 1 {
		return fmt.Errorf("multiple users found with same fingerprint: %s", fingerprint)
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
		return fmt.Errorf("failed to delete permission: %w", err)
	}

	// Check if any rows were affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("no permission found for email: %s", email)
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func handleWhoami(db *sql.DB, fingerprint string) (string, error) {
	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if we don't commit

	// First get the email for the current fingerprint
	var userEmails []string
	err = SELECT(table.SSHKeys.Email).
		FROM(table.SSHKeys).
		WHERE(table.SSHKeys.Fingerprint.EQ(String(fingerprint))).
		Query(tx, &userEmails)

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

	// Get all fingerprints and their registration times for this email
	type KeyInfo struct {
		Fingerprint string
		CreatedAt   int32
	}
	var keyInfos []KeyInfo
	err = SELECT(
		table.SSHKeys.Fingerprint.AS("key_info.fingerprint"),
		table.SSHKeys.CreatedAt.AS("key_info.created_at"),
	).FROM(
		table.SSHKeys,
	).WHERE(
		table.SSHKeys.Email.EQ(String(userEmail)),
	).ORDER_BY(
		table.SSHKeys.CreatedAt.ASC(),
	).Query(tx, &keyInfos)

	if err != nil {
		return "", fmt.Errorf("failed to query key info: %w", err)
	}

	// Get allowed users and their grant times
	var allowedUsers []struct {
		Email     string
		CreatedAt int32
	}
	err = SELECT(
		table.EmailPermissions.GranteeEmail.AS("email"),
		table.EmailPermissions.CreatedAt.AS("created_at"),
	).FROM(
		table.EmailPermissions,
	).WHERE(
		table.EmailPermissions.GranterEmail.EQ(String(userEmail)),
	).ORDER_BY(
		table.EmailPermissions.CreatedAt.ASC(),
	).Query(tx, &allowedUsers)

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
				user.Email,
				grantTime.Format(time.RFC3339)))
		}
	}

	return result.String(), nil
}

func handleUnregister(db *sql.DB, fingerprint string) error {
	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if we don't commit

	// First get the user's email
	var emails []string
	err = SELECT(table.SSHKeys.Email).
		FROM(table.SSHKeys).
		WHERE(table.SSHKeys.Fingerprint.EQ(String(fingerprint))).
		Query(tx, &emails)

	if err != nil {
		return fmt.Errorf("failed to query user email: %w", err)
	}
	if len(emails) == 0 {
		return fmt.Errorf("no registration found for this fingerprint")
	}
	if len(emails) > 1 {
		return fmt.Errorf("multiple registrations found for this fingerprint")
	}
	email := emails[0]

	// Count remaining keys for this email
	var keyCount []int64
	err = SELECT(COUNT(table.SSHKeys.Fingerprint)).
		FROM(table.SSHKeys).
		WHERE(table.SSHKeys.Email.EQ(String(email))).
		Query(tx, &keyCount)

	if err != nil {
		return fmt.Errorf("failed to count remaining keys: %w", err)
	}
	if len(keyCount) != 1 {
		return fmt.Errorf("failed to get key count")
	}

	// Only delete permissions if this is the last key
	if keyCount[0] == 1 {
		// Delete all permissions where this user is the granter
		_, err = table.EmailPermissions.DELETE().
			WHERE(table.EmailPermissions.GranterEmail.EQ(String(email))).
			Exec(tx)
		if err != nil {
			return fmt.Errorf("failed to delete granted permissions: %w", err)
		}

		// Delete all permissions where this user is the grantee
		_, err = table.EmailPermissions.DELETE().
			WHERE(table.EmailPermissions.GranteeEmail.EQ(String(email))).
			Exec(tx)
		if err != nil {
			return fmt.Errorf("failed to delete received permissions: %w", err)
		}
	}

	// Delete any pending verification codes for this fingerprint
	_, err = table.VerificationCodes.DELETE().
		WHERE(table.VerificationCodes.Fingerprint.EQ(String(fingerprint))).
		Exec(tx)
	if err != nil {
		return fmt.Errorf("failed to delete verification codes: %w", err)
	}

	// Delete the specific SSH key registration
	result, err := table.SSHKeys.DELETE().
		WHERE(table.SSHKeys.Fingerprint.EQ(String(fingerprint))).
		Exec(tx)
	if err != nil {
		return fmt.Errorf("failed to delete registration: %w", err)
	}

	// Verify that we actually deleted a registration
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("no registration found for this fingerprint")
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func handleGetEmail(db *sql.DB, callerFingerprint, targetFingerprint string) (string, error) {
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
