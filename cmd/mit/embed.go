//go:build !noembed

package main

import (
	"github.com/gabemeola/mit/command/embedcmd"
	"github.com/urfave/cli/v3"
)

func embedCommands() []*cli.Command {
	return []*cli.Command{embedcmd.IndexCommand(), embedcmd.SearchCommand()}
}
