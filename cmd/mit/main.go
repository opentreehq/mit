package main

import (
	"context"
	_ "embed"
	"os"
	"strings"

	"github.com/opentreehq/mit/command"
	"github.com/urfave/cli/v3"
)

//go:embed version.txt
var version string

func main() {
	commands := []*cli.Command{
		command.InitCommand(),
		command.CloneCommand(),
		command.AddCommand(),
		command.RemoveCommand(),
		command.ListCommand(),
		command.DoctorCommand(),
		command.DiscoverCommand(),
		command.DepsCommand(),
		command.ContextCommand(),
		command.StatusCommand(),
		command.SyncCommand(),
		command.PullCommand(),
		command.PushCommand(),
		command.FetchCommand(),
		command.SwitchCommand(),
		command.BranchCommand(),
		command.CommitCommand(),
		command.DiffCommand(),
		command.LogCommand(),
		command.GrepCommand(),
		command.RunCommand(),
		command.WorktreeCommand(),
		command.TaskCommand(),
		command.MemoryCommand(),
		command.SkillCommand(),
		command.GuideCommand("mit"),
	}
	commands = append(commands, embedCommands()...)

	app := &cli.Command{
		Name:                  "mit",
		Usage:                 "Multi-repo Integration Tool",
		Version:               strings.TrimSpace(version),
		Flags:                 command.GlobalFlags(),
		Commands:              commands,
		EnableShellCompletion: true,
	}
	if err := app.Run(context.Background(), os.Args); err != nil {
		os.Exit(1)
	}
}
