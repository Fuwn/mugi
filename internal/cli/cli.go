package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ebisu/mugi/internal/remote"
)

type Command struct {
	Operation  remote.Operation
	Repo       string
	Remotes    []string
	ConfigPath string
	Verbose    bool
	Help       bool
	Version    bool
}

var ErrUnknownCommand = errors.New("unknown command")

func Parse(args []string) (Command, error) {
	cmd := Command{
		Remotes: []string{remote.All},
	}

	if len(args) == 0 {
		cmd.Help = true

		return cmd, nil
	}

	args, cmd.ConfigPath = extractConfigFlag(args)
	args, cmd.Verbose = extractVerboseFlag(args)

	for _, arg := range args {
		if arg == "-h" || arg == "--help" || arg == "help" {
			cmd.Help = true

			return cmd, nil
		}

		if arg == "-v" || arg == "--version" || arg == "version" {
			cmd.Version = true

			return cmd, nil
		}
	}

	if len(args) == 0 {
		cmd.Help = true

		return cmd, nil
	}

	switch args[0] {
	case "pull":
		cmd.Operation = remote.Pull
	case "push":
		cmd.Operation = remote.Push
	case "fetch":
		cmd.Operation = remote.Fetch
	default:
		return cmd, fmt.Errorf("%w: %s", ErrUnknownCommand, args[0])
	}

	remaining := args[1:]

	if len(remaining) == 0 {
		cmd.Repo = remote.All

		return cmd, nil
	}

	cmd.Repo = remaining[0]
	remaining = remaining[1:]

	if len(remaining) > 0 {
		cmd.Remotes = remaining
	}

	return cmd, nil
}

func Usage() string {
	return `Mugi - Personal Multi-Git Remote Manager

Usage:
  mugi [flags] <command> [repo] [remotes...]

Commands:
  pull    Pull from remote(s)
  push    Push to remote(s)
  fetch   Fetch from remote(s)
  help    Show this help
  version Show version

Flags:
  -c, --config <path>  Override config file path
  -V, --verbose        Show detailed output

Examples:
  mugi pull                      Pull all repositories from all remotes
  mugi pull windmark             Pull Windmark from all remotes
  mugi pull windmark github      Pull Windmark from GitHub only
  mugi push windmark gh cb       Push Windmark to GitHub and Codeberg
  mugi fetch gemrest/september   Fetch specific repository
  mugi -c ./test.yaml pull       Use custom config

Config: ` + configPath()
}

func configPath() string {
	xdg := os.Getenv("XDG_CONFIG_HOME")

	if xdg == "" {
		home, _ := os.UserHomeDir()

		return home + "/.config/mugi/config.yaml"
	}

	return xdg + "/mugi/config.yaml"
}

func extractConfigFlag(args []string) ([]string, string) {
	var remaining []string
	var configPath string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "-c" || arg == "--config" {
			if i+1 < len(args) {
				configPath = args[i+1]
				i++
			}

			continue
		}

		if v, ok := strings.CutPrefix(arg, "--config="); ok {
			configPath = v

			continue
		}

		if v, ok := strings.CutPrefix(arg, "-c"); ok && v != "" {
			configPath = v

			continue
		}

		remaining = append(remaining, arg)
	}

	return remaining, configPath
}

func extractVerboseFlag(args []string) ([]string, bool) {
	var remaining []string
	var verbose bool

	for _, arg := range args {
		if arg == "-V" || arg == "--verbose" {
			verbose = true

			continue
		}

		remaining = append(remaining, arg)
	}

	return remaining, verbose
}
