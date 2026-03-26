package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gabemeola/mit/internal/output"
	"github.com/gabemeola/mit/internal/workspace"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var depsCmd = &cobra.Command{
	Use:   "deps",
	Short: "Analyze inter-repo dependencies",
	Long:  "Scans package.json and docker-compose.yml files to find cross-repo dependencies.",
	RunE:  runDeps,
}

func init() {
	rootCmd.AddCommand(depsCmd)
}

type dependency struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Type    string `json:"type"` // "npm" or "docker-compose"
	Detail  string `json:"detail,omitempty"`
}

type depsResult struct {
	Repo         string       `json:"repo"`
	Dependencies []dependency `json:"dependencies"`
}

func runDeps(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	ws, err := workspace.Load(dir)
	if err != nil {
		return err
	}

	sel := workspace.NewSelector(flagRepos, flagExclude)
	repos := ws.FilterRepos(sel)

	// Build a set of known repo names for cross-reference detection.
	repoNames := make(map[string]bool)
	for _, repo := range repos {
		repoNames[repo.Name] = true
	}

	var allDeps []dependency
	var results []depsResult

	for _, repo := range repos {
		if !repo.Exists {
			continue
		}

		var repoDeps []dependency

		// Scan package.json
		npmDeps := scanPackageJSON(repo.AbsPath, repo.Name, repoNames)
		repoDeps = append(repoDeps, npmDeps...)

		// Scan docker-compose.yml / docker-compose.yaml
		composeDeps := scanDockerCompose(repo.AbsPath, repo.Name, repoNames)
		repoDeps = append(repoDeps, composeDeps...)

		if len(repoDeps) > 0 {
			results = append(results, depsResult{
				Repo:         repo.Name,
				Dependencies: repoDeps,
			})
			allDeps = append(allDeps, repoDeps...)
		}
	}

	if getOutputFormat() == "json" {
		env := output.NewEnvelope("deps", results)
		env.Summary = map[string]int{
			"repos_scanned":    len(repos),
			"dependencies":     len(allDeps),
		}
		return output.New("json").Format(env)
	}

	if len(allDeps) == 0 {
		fmt.Println("No inter-repo dependencies found.")
		return nil
	}

	headers := []string{"FROM", "TO", "TYPE", "DETAIL"}
	var rows [][]string
	for _, d := range allDeps {
		rows = append(rows, []string{d.From, d.To, d.Type, d.Detail})
	}
	output.PrintTable(os.Stdout, headers, rows)
	return nil
}

// scanPackageJSON looks for cross-repo references in package.json dependencies.
func scanPackageJSON(repoPath, repoName string, knownRepos map[string]bool) []dependency {
	pkgPath := filepath.Join(repoPath, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return nil
	}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}

	var deps []dependency
	allPkgDeps := mergeMaps(pkg.Dependencies, pkg.DevDependencies)
	for name, version := range allPkgDeps {
		// Check if the package name matches a known repo name or contains a reference.
		baseName := name
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			baseName = name[idx+1:]
		}
		if baseName != repoName && knownRepos[baseName] {
			deps = append(deps, dependency{
				From:   repoName,
				To:     baseName,
				Type:   "npm",
				Detail: fmt.Sprintf("%s@%s", name, version),
			})
		}
	}
	return deps
}

// scanDockerCompose looks for cross-repo service references in docker-compose files.
func scanDockerCompose(repoPath, repoName string, knownRepos map[string]bool) []dependency {
	var deps []dependency

	for _, fname := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"} {
		composePath := filepath.Join(repoPath, fname)
		data, err := os.ReadFile(composePath)
		if err != nil {
			continue
		}

		var compose struct {
			Services map[string]struct {
				DependsOn interface{} `yaml:"depends_on"`
				Image     string      `yaml:"image"`
				Build     interface{} `yaml:"build"`
			} `yaml:"services"`
		}
		if err := yaml.Unmarshal(data, &compose); err != nil {
			continue
		}

		for _, svc := range compose.Services {
			depNames := extractDependsOn(svc.DependsOn)
			for _, depName := range depNames {
				if depName != repoName && knownRepos[depName] {
					deps = append(deps, dependency{
						From:   repoName,
						To:     depName,
						Type:   "docker-compose",
						Detail: fname,
					})
				}
			}
		}
	}
	return deps
}

// extractDependsOn handles both list and map forms of depends_on.
func extractDependsOn(v interface{}) []string {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case []interface{}:
		var names []string
		for _, item := range val {
			if s, ok := item.(string); ok {
				names = append(names, s)
			}
		}
		return names
	case map[string]interface{}:
		var names []string
		for name := range val {
			names = append(names, name)
		}
		return names
	}
	return nil
}

func mergeMaps(maps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}
