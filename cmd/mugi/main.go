package main

import (
	"fmt"
	"os"

	"github.com/ebisu/mugi/internal/cli"
	"github.com/ebisu/mugi/internal/config"
	"github.com/ebisu/mugi/internal/ui"
)

const version = "0.1.0"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cmd, err := cli.Parse(os.Args[1:])
	if err != nil {
		return err
	}

	if cmd.Help {
		fmt.Println(cli.Usage())

		return nil
	}

	if cmd.Version {
		fmt.Printf("mugi %s\n", version)

		return nil
	}

	cfg, err := config.Load(cmd.ConfigPath)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	tasks := ui.BuildTasks(cfg, cmd.Repo, cmd.Remotes)
	if len(tasks) == 0 {
		return fmt.Errorf("no matching repositories or remotes found")
	}

	return ui.Run(cmd.Operation, tasks, cmd.Verbose, cmd.Force)
}
