package command

import (
	"fmt"
	"runtime"

	"github.com/urfave/cli/v3"
)

func GlobalFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "repos", Usage: "filter to specific repos (comma-separated)"},
		&cli.StringFlag{Name: "exclude", Usage: "exclude specific repos (comma-separated)"},
		&cli.IntFlag{Name: "jobs", Aliases: []string{"j"}, Value: runtime.NumCPU(), Usage: "parallelism (default: num CPUs)"},
		&cli.StringFlag{Name: "output", Value: "table", Usage: "output format: json, table, plain"},
		&cli.BoolFlag{Name: "quiet", Aliases: []string{"q"}, Usage: "suppress progress output"},
		&cli.BoolFlag{Name: "dry-run", Usage: "show what would be done without doing it"},
	}
}

func getOutputFormat(cmd *cli.Command) string {
	return cmd.String("output")
}

func getParallelism(cmd *cli.Command) int {
	j := cmd.Int("jobs")
	if j <= 0 {
		return runtime.NumCPU()
	}
	return int(j)
}

func isDryRun(cmd *cli.Command) bool {
	return cmd.Bool("dry-run")
}

func isQuiet(cmd *cli.Command) bool {
	return cmd.Bool("quiet")
}

func dryRunMsg(cmd *cli.Command, format string, args ...any) {
	if isDryRun(cmd) {
		fmt.Printf("[dry-run] "+format+"\n", args...)
	}
}
