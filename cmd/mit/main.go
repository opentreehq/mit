package main

import (
	_ "embed"
	"os"
	"strings"

	"github.com/gabemeola/mit/internal/cli"
)

//go:embed version.txt
var version string

func main() {
	cli.SetVersion(strings.TrimSpace(version))
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
