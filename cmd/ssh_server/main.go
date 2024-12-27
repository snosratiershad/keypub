package main

import (
	"fmt"
	"io"
	"log"
	"time"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"

	"keypub/internal/mail"
	rl "keypub/internal/ratelimit"

	"database/sql"
	_ "github.com/mattn/go-sqlite3"

	db_utils "keypub/internal/db"
)

const (
	hostKeyPath          = "/home/ubuntu/.keys/.host"
	port                 = 22
	rl_limit             = 600
	rl_duration          = 10 * time.Hour
	rl_strict            = false
	db_fname             = "/home/ubuntu/data/keysdb.sqlite3"
	resendKeyPath        = "/home/ubuntu/.keys/.resend"
	confirmationFromMail = "confirmations@keypub.sh"
	confirmationFromName = "keypub.sh"
	verificationDuration = 1 * time.Hour
)

func main() {
	ratelimit := rl.NewRateLimiter(rl_limit, rl_duration, rl_strict)
	defer ratelimit.Stop()

	hostKey, err := loadHostKey(hostKeyPath)
	if err != nil {
		log.Fatal(err)
	}

	server := ssh.Server{
		Addr:        fmt.Sprintf(":%d", port),
		HostSigners: []ssh.Signer{hostKey},
		PublicKeyHandler: func(ctx ssh.Context, key ssh.PublicKey) bool {
			return true
		},
	}

	db, err := sql.Open("sqlite3", db_fname)
	if err != nil {
		log.Fatalf("Cannot open db file: %s", err)
	}
	defer db.Close()

	verification_cleaner := db_utils.NewVerificationCleaner(db, verificationDuration)
	defer verification_cleaner.Close()

	mail_sender, err := mail.NewMailSender(resendKeyPath, confirmationFromMail, confirmationFromName)
	if err != nil {
		log.Fatalf("Could not initialize MailSender: %s", err)
	}

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

	log.Printf("Starting SSH server on port %d...", port)
	log.Fatal(server.ListenAndServe())
}
