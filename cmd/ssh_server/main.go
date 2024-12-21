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
	port                 = 2223
	rl_limit             = 600
	rl_duration          = 3 * time.Hour
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

	db, err := sql.Open("sqlite3", "/home/ubuntu/data/keysdb.sqlite3")
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

		switch args[0] {
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
		case "add":
			if len(args) != 2 {
				io.WriteString(s, "Usage: add <field:value>\n")
				return
			}
			handleAdd(s, args[1])
		case "get":
			if len(args) < 3 {
				io.WriteString(s, "Usage: get <field> from <key>\n")
				return
			}
			handleGet(s, args[1], args[2])
		default:
			io.WriteString(s, fmt.Sprintf("Unknown command: %s\n", args[0]))
		}
	})

	log.Printf("Starting SSH server on port %d...", port)
	log.Fatal(server.ListenAndServe())
}
