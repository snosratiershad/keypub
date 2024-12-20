package main

import (
	"fmt"
	"io"
	"log"
	"time"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"

	rl "keypub/internal/ratelimit"
)

const (
	hostKeyPath = "/home/ubuntu/.keys/.host"
	port        = 2223
	rl_limit    = 600
	rl_duration = 3 * time.Hour
	rl_strict   = false
	db_fname    = "/home/ubuntu/data/keysdb.sqlite3"
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

	server.Handle(func(s ssh.Session) {
		fingerprint := gossh.FingerprintSHA256(s.PublicKey())
		rl_res := ratelimit.Check(fingerprint)
		if !rl_res.Allowed {
			io.WriteString(s, rl_res.NextTime.String())
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
			handleRegister(s, args[1])
		case "confirm":
			if len(args) != 2 {
				io.WriteString(s, "Usage: confirm <code>\n")
				return
			}
			handleConfirm(s, args[1])
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
