package command

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gabemeola/mit/config"
	"github.com/gabemeola/mit/output"
	"github.com/gabemeola/mit/skills"
	"github.com/urfave/cli/v3"
)

// SkillCommand returns the skill command with list, show, create, and search subcommands.
func SkillCommand() *cli.Command {
	return &cli.Command{
		Name:        "skill",
		Usage:       "Manage workspace skills (reusable procedures and playbooks)",
		Description: "Manage workspace skills (reusable procedures and playbooks).",
		Commands: []*cli.Command{
			{
				Name:   "list",
				Usage:  "List all skills",
				Action: skillListAction,
			},
			{
				Name:   "show",
				Usage:  "Show a skill's details",
				Action: skillShowAction,
			},
			{
				Name:  "create",
				Usage: "Create a new skill",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "description", Usage: "skill description", Local: true},
					&cli.StringFlag{Name: "triggers", Usage: "comma-separated trigger keywords", Local: true},
					&cli.StringFlag{Name: "skill-repos", Usage: "comma-separated repo names this skill applies to", Local: true},
					&cli.StringFlag{Name: "content", Usage: "skill content/instructions", Local: true},
				},
				Action: skillCreateAction,
			},
			{
				Name:   "search",
				Usage:  "Search skills by keyword",
				Action: skillSearchAction,
			},
		},
	}
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

func skillListAction(_ context.Context, cmd *cli.Command) error {
	reg, err := openSkillRegistry()
	if err != nil {
		return err
	}

	all, err := reg.List()
	if err != nil {
		return err
	}

	if getOutputFormat(cmd) == "json" {
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

func skillShowAction(_ context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 1 {
		return fmt.Errorf("expected 1 argument(s)")
	}
	reg, err := openSkillRegistry()
	if err != nil {
		return err
	}

	skill, err := reg.Get(cmd.Args().First())
	if err != nil {
		return err
	}

	if getOutputFormat(cmd) == "json" {
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

func skillCreateAction(_ context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 1 {
		return fmt.Errorf("expected 1 argument(s)")
	}
	reg, err := openSkillRegistry()
	if err != nil {
		return err
	}

	skill := &skills.Skill{
		Name:        cmd.Args().First(),
		Description: cmd.String("description"),
		Content:     cmd.String("content"),
	}

	if triggers := cmd.String("triggers"); triggers != "" {
		for _, t := range strings.Split(triggers, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				skill.Triggers = append(skill.Triggers, t)
			}
		}
	}
	if skillRepos := cmd.String("skill-repos"); skillRepos != "" {
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

	if getOutputFormat(cmd) == "json" {
		env := output.NewEnvelope("skill.create", skill)
		return output.New("json").Format(env)
	}

	fmt.Printf("Created skill %q\n", skill.Name)
	return nil
}

func skillSearchAction(_ context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 1 {
		return fmt.Errorf("expected 1 argument(s)")
	}
	reg, err := openSkillRegistry()
	if err != nil {
		return err
	}

	results, err := reg.Search(cmd.Args().First())
	if err != nil {
		return err
	}

	if getOutputFormat(cmd) == "json" {
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
