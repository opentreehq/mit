package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	flagRepos   string
	flagExclude string
	flagJobs    int
	flagOutput  string
	flagQuiet   bool
	flagDryRun  bool
)

var rootCmd = &cobra.Command{
	Use:   "mit",
	Short: "Multi-repo Integration Tool",
	Long: `mit is a multi-repo management tool that handles multiple repositories
without git submodules. It supports both git and Sapling (sl) as VCS drivers
and is designed for both humans and AI agents.`,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagRepos, "repos", "", "filter to specific repos (comma-separated)")
	rootCmd.PersistentFlags().StringVar(&flagExclude, "exclude", "", "exclude specific repos (comma-separated)")
	rootCmd.PersistentFlags().IntVarP(&flagJobs, "jobs", "j", runtime.NumCPU(), "parallelism (default: num CPUs)")
	rootCmd.PersistentFlags().StringVar(&flagOutput, "output", "table", "output format: json, table, plain")
	rootCmd.PersistentFlags().BoolVarP(&flagQuiet, "quiet", "q", false, "suppress progress output")
	rootCmd.PersistentFlags().BoolVar(&flagDryRun, "dry-run", false, "show what would be done without doing it")
}

func Execute() error {
	return rootCmd.Execute()
}

func getOutputFormat() string {
	return flagOutput
}

func getParallelism() int {
	if flagJobs <= 0 {
		return runtime.NumCPU()
	}
	return flagJobs
}

func dryRunMsg(format string, args ...any) {
	if flagDryRun {
		fmt.Printf("[dry-run] "+format+"\n", args...)
	}
}
