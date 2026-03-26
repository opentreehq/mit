package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/gabemeola/mit/internal/config"
	"github.com/gabemeola/mit/internal/output"
	"github.com/gabemeola/mit/internal/skills"
	"github.com/spf13/cobra"
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Manage workspace skills (reusable procedures and playbooks)",
}

var skillListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all skills",
	RunE:  runSkillList,
}

var skillShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show a skill's details",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillShow,
}

var skillCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new skill",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillCreate,
}

var skillSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search skills by keyword",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillSearch,
}

var (
	skillDescription string
	skillTriggers    string
	skillRepos       string
	skillContent     string
)

func init() {
	skillCreateCmd.Flags().StringVar(&skillDescription, "description", "", "skill description")
	skillCreateCmd.Flags().StringVar(&skillTriggers, "triggers", "", "comma-separated trigger keywords")
	skillCreateCmd.Flags().StringVar(&skillRepos, "skill-repos", "", "comma-separated repo names this skill applies to")
	skillCreateCmd.Flags().StringVar(&skillContent, "content", "", "skill content/instructions")

	skillCmd.AddCommand(skillListCmd, skillShowCmd, skillCreateCmd, skillSearchCmd)
	rootCmd.AddCommand(skillCmd)
}

func openSkillRegistry() (*skills.Registry, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	root, err := config.FindRoot(dir)
	if err != nil {
		return nil, err
	}
	return skills.NewRegistry(root)
}

func runSkillList(cmd *cobra.Command, args []string) error {
	reg, err := openSkillRegistry()
	if err != nil {
		return err
	}

	all, err := reg.List()
	if err != nil {
		return err
	}

	if getOutputFormat() == "json" {
		env := output.NewEnvelope("skill.list", all)
		env.Summary = map[string]int{"total": len(all)}
		return output.New("json").Format(env)
	}

	if len(all) == 0 {
		fmt.Println("No skills found.")
		return nil
	}

	headers := []string{"NAME", "DESCRIPTION", "TRIGGERS"}
	var rows [][]string
	for _, s := range all {
		rows = append(rows, []string{
			s.Name,
			s.Description,
			strings.Join(s.Triggers, ", "),
		})
	}
	output.PrintTable(os.Stdout, headers, rows)
	return nil
}

func runSkillShow(cmd *cobra.Command, args []string) error {
	reg, err := openSkillRegistry()
	if err != nil {
		return err
	}

	skill, err := reg.Get(args[0])
	if err != nil {
		return err
	}

	if getOutputFormat() == "json" {
		env := output.NewEnvelope("skill.show", skill)
		return output.New("json").Format(env)
	}

	fmt.Printf("Name:        %s\n", skill.Name)
	fmt.Printf("Description: %s\n", skill.Description)
	if len(skill.Triggers) > 0 {
		fmt.Printf("Triggers:    %s\n", strings.Join(skill.Triggers, ", "))
	}
	if len(skill.Repos) > 0 {
		fmt.Printf("Repos:       %s\n", strings.Join(skill.Repos, ", "))
	}
	if skill.Content != "" {
		fmt.Printf("\n%s\n", skill.Content)
	}
	return nil
}

func runSkillCreate(cmd *cobra.Command, args []string) error {
	reg, err := openSkillRegistry()
	if err != nil {
		return err
	}

	skill := &skills.Skill{
		Name:        args[0],
		Description: skillDescription,
		Content:     skillContent,
	}

	if skillTriggers != "" {
		for _, t := range strings.Split(skillTriggers, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				skill.Triggers = append(skill.Triggers, t)
			}
		}
	}
	if skillRepos != "" {
		for _, r := range strings.Split(skillRepos, ",") {
			r = strings.TrimSpace(r)
			if r != "" {
				skill.Repos = append(skill.Repos, r)
			}
		}
	}

	if err := reg.Create(skill); err != nil {
		return err
	}

	if getOutputFormat() == "json" {
		env := output.NewEnvelope("skill.create", skill)
		return output.New("json").Format(env)
	}

	fmt.Printf("Created skill %q\n", skill.Name)
	return nil
}

func runSkillSearch(cmd *cobra.Command, args []string) error {
	reg, err := openSkillRegistry()
	if err != nil {
		return err
	}

	results, err := reg.Search(args[0])
	if err != nil {
		return err
	}

	if getOutputFormat() == "json" {
		env := output.NewEnvelope("skill.search", results)
		env.Summary = map[string]int{"total": len(results)}
		return output.New("json").Format(env)
	}

	if len(results) == 0 {
		fmt.Println("No skills matched.")
		return nil
	}

	headers := []string{"NAME", "DESCRIPTION", "TRIGGERS"}
	var rows [][]string
	for _, s := range results {
		rows = append(rows, []string{
			s.Name,
			s.Description,
			strings.Join(s.Triggers, ", "),
		})
	}
	output.PrintTable(os.Stdout, headers, rows)
	return nil
}
