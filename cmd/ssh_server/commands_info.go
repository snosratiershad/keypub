package main

import (
	cmd "keypub/internal/command"

	_ "github.com/mattn/go-sqlite3"
)

func registerCommandInfo(registry *cmd.CommandRegistry) *cmd.CommandRegistry {

	registry.Register(cmd.Command{
		Name:        "help",
		Usage:       "help",
		Description: "Understand the motivation behind this project and how it helps solve common SSH key management challenges.",
		Category:    "Info",
		Handler: func(ctx *cmd.CommandContext) (info string, err error) {
			return registry.GetHelpText(), nil
		},
	})
	registry.Register(cmd.Command{
		Name:        "about",
		Usage:       "about",
		Description: "Learn about this service and how it helps map SSH keys to email addresses while protecting user privacy.",
		Category:    "Info",
		Handler: func(ctx *cmd.CommandContext) (info string, err error) {
			return `* Verified registry linking SSH public keys to email addresses
* No installation or configuration needed - works with your existing SSH setup
* Privacy-focused: you control what information is public or private
* Simple email verification process
* Free public service`, nil
		},
	})
	registry.Register(cmd.Command{
		Name:        "why",
		Usage:       "why",
		Description: "Understand the motivation behind this project and how it helps solve common SSH key management challenges.",
		Category:    "Info",
		Handler: func(ctx *cmd.CommandContext) (info string, err error) {
			return `* Single verified identity for all SSH-based applications - register once, use everywhere
* Perfect for SSH application developers - no need to build and maintain user verification systems
* Users control their privacy - they decide which applications can access their email
* Lightweight alternative to OAuth for CLI applications - just use SSH keys that users already have
* Central identity system that respects privacy and puts users in control`, nil
		},
	})

	return registry
}
