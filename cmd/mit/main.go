package main

import (
	"os"

	"github.com/gabemeola/mit/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
