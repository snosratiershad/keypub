package main

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"

	"keypub/internal/config"
	"keypub/internal/mail"
	rl "keypub/internal/ratelimit"

	_ "github.com/mattn/go-sqlite3"

	db_utils "keypub/internal/db"
)

func main() {
	// Load configuration
	result, err := config.LoadFromFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		config.PrintUsage()
		os.Exit(1)
	}

	cfg := result.Config
	log.Printf("Starting server with %s", result.Source)

	// initialize rate limiter
	ratelimit := rl.NewRateLimiter(cfg.RateLimit.Limit, cfg.RateLimit.Duration, cfg.RateLimit.Strict)
	defer ratelimit.Stop()

	// initialize server
	hostKey, err := loadHostKey(cfg.Server.HostKey)
	if err != nil {
		log.Fatal(err)
	}

	server := ssh.Server{
		Addr:        fmt.Sprintf(":%d", cfg.Server.Port),
		HostSigners: []ssh.Signer{hostKey},
		PublicKeyHandler: func(ctx ssh.Context, key ssh.PublicKey) bool {
			return true
		},
	}

	// open DB
	db, err := db_utils.NewDB(cfg.Database.Path)
	if err != nil {
		log.Fatalf("Cannot open db file: %s", err)
	}
	defer db.Close()

	// regular interval DB cleaner (currently only for verification codes)
	verification_cleaner := db_utils.NewVerificationCleaner(db, cfg.Verification.Duration)
	defer verification_cleaner.Close()

	// initialize mail sender
	mail_sender, err := mail.NewMailSender(cfg.Email.ResendKeyPath, cfg.Email.FromEmail, cfg.Email.FromName)
	if err != nil {
		log.Fatalf("Could not initialize MailSender: %s", err)
	}

	// Only initialize backup if enabled
	if cfg.Backup.Enabled {
		bm, err := initializeBackup(cfg, db)
		if err != nil {
			log.Fatalf("could not create the backup manager: %s", err)
		}
		bm.Start()
		defer bm.Stop()
	}

	// start main ssh server
	server.Handle(func(s ssh.Session) {
		fingerprint := gossh.FingerprintSHA256(s.PublicKey())
		rl_res := ratelimit.Check(fingerprint)
		if !rl_res.Allowed {
			io.WriteString(s, "Error: Rate-limited\n")
			return
		}
		args := s.Command()
		if len(args) < 1 {
			io.WriteString(s, "Error: Command required\n")
			return
		}

		comm := args[0]
		switch comm {
		case "register":
			if len(args) != 2 {
				io.WriteString(s, "Usage: register <email>\n")
				return
			}
			email := args[1]
			err := mail.ValidateEmail(email)
			if err != nil {
				io.WriteString(s, "Error: Mail address fails validation\n")
				return
			}
			err = handleRegister(db, mail_sender, email, fingerprint)
			if err != nil {
				io.WriteString(s, fmt.Sprintf("Error: %s\n", err))
			} else {
				io.WriteString(s, "Success: Confirmation mail sent, valid for 1hr\n")
			}
		case "confirm":
			if len(args) != 2 {
				io.WriteString(s, "Usage: confirm <code>\n")
				return
			}
			code := args[1]
			err, mail := handleConfirm(db, fingerprint, code)
			if err != nil {
				io.WriteString(s, fmt.Sprintf("Error: %s\n", err))
			} else {
				io.WriteString(s, fmt.Sprintf("Success: Mail %s is now associated with fingerprint %s\n", mail, fingerprint))
			}
		case "allow":
			if len(args) != 2 {
				io.WriteString(s, "Usage: allow <email>\n")
				return
			}

			email := args[1]
			err := mail.ValidateEmail(email)
			if err != nil {
				io.WriteString(s, "Error: Mail address fails validation\n")
				return
			}
			err = handleAllow(db, email, fingerprint)
			if err != nil {
				io.WriteString(s, fmt.Sprintf("Error: %s\n", err))
			} else {
				io.WriteString(s, fmt.Sprintf("Success: user %s can read your email address\n", email))
			}
		case "deny":
			if len(args) != 2 {
				io.WriteString(s, "Usage: deny <email>\n")
				return
			}

			email := args[1]
			err := mail.ValidateEmail(email)
			if err != nil {
				io.WriteString(s, "Error: Mail address fails validation\n")
				return
			}
			err = handleDeny(db, email, fingerprint)
			if err != nil {
				io.WriteString(s, fmt.Sprintf("Error: %s\n", err))
			} else {
				io.WriteString(s, fmt.Sprintf("Success: user %s can no longer read your email address\n", email))
			}
		case "whoami":
			info, err := handleWhoami(db, fingerprint)
			if err != nil {
				io.WriteString(s, fmt.Sprintf("Error: %s\n", err))
			} else {
				io.WriteString(s, fmt.Sprintf("%s\n", info))
			}
		case "get":
			if len(args) != 3 || args[1] != "email" || args[2] == "" {
				io.WriteString(s, "Usage: get email <fingerprint>\n")
				return
			}
			targetFingerprint := args[2]
			email, err := handleGetEmail(db, fingerprint, targetFingerprint)
			if err != nil {
				io.WriteString(s, fmt.Sprintf("Error: %s\n", err))
			} else {
				io.WriteString(s, fmt.Sprintf("%s\n", email))
			}
		case "unregister":
			err := handleUnregister(db, fingerprint)
			if err != nil {
				io.WriteString(s, fmt.Sprintf("Error: %s\n", err))
			} else {
				io.WriteString(s, "Success: Your registration and all related permissions have been removed\n")
			}
		case "help":
			io.WriteString(s, `Available commands:

register <email>
   Register your SSH key with the given email address.
   You will receive a confirmation code via email.

confirm <code> 
   Confirm your email address using the code you received.
   This completes your registration.

allow <email>
   Grant permission to the given email address to see your email.
   The user must be registered in the system.

deny <email>
   Remove permission for the given email address to see your email.

whoami
   Show your fingerprint, registered email, registration date,
   and list of users allowed to see your email.

get email <fingerprint>  
   Get the email address associated with the given fingerprint.
   Only works if you have permission to see it.

unregister
   Remove your registration and all associated permissions.
   This cannot be undone.

about
   Learn about this service and how it helps map SSH keys 
   to email addresses while protecting user privacy.

why
   Understand the motivation behind this project and how it 
   helps solve common SSH key management challenges.

help
   Show this help message.
`)
		case "why":
			io.WriteString(s, `* Single verified identity for all SSH-based applications - register once, use everywhere
* Perfect for SSH application developers - no need to build and maintain user verification systems
* Users control their privacy - they decide which applications can access their email
* Lightweight alternative to OAuth for CLI applications - just use SSH keys that users already have
* Central identity system that respects privacy and puts users in control
`)
		case "about":
			io.WriteString(s, `* Verified registry linking SSH public keys to email addresses
* No installation or configuration needed - works with your existing SSH setup
* Privacy-focused: you control what information is public or private
* Simple email verification process
* Free public service
`)
		default:
			io.WriteString(s, fmt.Sprintf("Unknown command: %s\n", args[0]))
		}
	})

	log.Printf("Starting SSH server on port %d...", cfg.Server.Port)
	log.Fatal(server.ListenAndServe())
}

func initializeBackup(cfg *config.Config, db *sql.DB) (*db_utils.BackupManager, error) {
	s3access, err := os.ReadFile(cfg.Backup.S3AccessPath)
	if err != nil {
		return nil, fmt.Errorf("cannot load s3 access key: %v", err)
	}
	s3secret, err := os.ReadFile(cfg.Backup.S3SecretPath)
	if err != nil {
		return nil, fmt.Errorf("cannot load s3 secret: %v", err)
	}
	s3endpoint, err := os.ReadFile(cfg.Backup.S3EndpointPath)
	if err != nil {
		return nil, fmt.Errorf("cannot load s3 endpoint: %v", err)
	}

	return db_utils.NewBackupManager(db_utils.BackupConfig{
		DB: db,
		S3Creds: db_utils.S3Credentials{
			Region:          cfg.Backup.S3Region,
			AccessKeyID:     string(s3access),
			SecretAccessKey: string(s3secret),
			Endpoint:        string(s3endpoint),
		},
		BucketName:     cfg.Backup.BucketName,
		BackupDelta:    cfg.Backup.Delta,
		RetentionCount: cfg.Backup.RetentionCount,
		TempDir:        cfg.Backup.TempDir,
		BackupLabel:    cfg.Backup.Label,
	})
}
