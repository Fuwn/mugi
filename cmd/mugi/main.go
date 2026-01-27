package main

import (
	"fmt"
	"os"

	"github.com/ebisu/mugi/internal/cli"
	"github.com/ebisu/mugi/internal/config"
	"github.com/ebisu/mugi/internal/manage"
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

	configPath := cmd.ConfigPath
	if configPath == "" {
		configPath, _ = config.Path()
	}

	switch cmd.Type {
	case cli.CommandAdd:
		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("config: %w", err)
		}

		if err := manage.Add(cmd.Path, configPath, cfg.Remotes); err != nil {
			return err
		}

		fmt.Printf("Added repository: %s\n", cmd.Path)

		return nil

	case cli.CommandRemove:
		if err := manage.Remove(cmd.Repo, configPath); err != nil {
			return err
		}

		fmt.Printf("Removed repository: %s\n", cmd.Repo)

		return nil

	case cli.CommandList:
		repos, err := manage.List(configPath)
		if err != nil {
			return err
		}

		for _, repo := range repos {
			fmt.Printf("%s (%s)\n", repo.Name, repo.Path)
		}

		return nil
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	applyDefaults(&cmd, cfg)

	tasks := ui.BuildTasks(cfg, cmd.Repo, cmd.Remotes)
	if len(tasks) == 0 {
		return fmt.Errorf("no matching repositories or remotes found")
	}

	return ui.Run(cmd.Operation, tasks, cmd.Verbose, cmd.Force, cmd.Linear)
}

func applyDefaults(cmd *cli.Command, cfg config.Config) {
	if cfg.Defaults.Verbose {
		cmd.Verbose = true
	}

	if cfg.Defaults.Linear {
		cmd.Linear = true
	}

	if len(cmd.Remotes) == 1 && cmd.Remotes[0] == "all" {
		opRemotes := cfg.Defaults.RemotesFor(cmd.Operation.String())

		if len(opRemotes) > 0 {
			cmd.Remotes = opRemotes
		}
	}
}
