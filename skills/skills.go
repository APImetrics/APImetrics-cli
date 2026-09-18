package skills

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"apicontext.com/apimetrics/cli"
	"github.com/spf13/cobra"
)

//go:embed embed/*.md
var skillFiles embed.FS

// claudeSkillFiles holds the current-format Claude Code skills, one directory
// per skill containing a SKILL.md. These are installed by `--claude-skills`
// into .claude/skills/<name>/SKILL.md; the legacy flat setup skills in
// skillFiles are left untouched.
//
//go:embed claude-skills
var claudeSkillFiles embed.FS

type agent int

const (
	agentCustom agent = iota
	agentClaudeCode
	agentClaudeSkills
	agentCodex
)

// Init registers the skills command group and the top-level onboard command.
func Init(cmd *cobra.Command) {
	skills := cobra.Command{
		Use:   "skills",
		Short: "Manage agent skills for APImetrics",
	}

	install := cobra.Command{
		Use:   "install",
		Short: "Install APImetrics skills into an agent skills directory",
		Example: "  " + os.Args[0] + " skills install --claude-skills\n" +
			"  " + os.Args[0] + " skills install --claude-code\n" +
			"  " + os.Args[0] + " skills install --codex\n" +
			"  " + os.Args[0] + " skills install --dir ./custom",
		RunE: func(cmd *cobra.Command, args []string) error {
			claudeCode, _ := cmd.Flags().GetBool("claude-code")
			claudeSkills, _ := cmd.Flags().GetBool("claude-skills")
			codex, _ := cmd.Flags().GetBool("codex")
			dir, _ := cmd.Flags().GetString("dir")

			target, kind, err := resolveTarget(claudeCode, claudeSkills, codex, dir)
			if err != nil {
				return err
			}

			return installSkills(target, kind)
		},
	}

	install.Flags().Bool("claude-skills", false, "Install into .claude/skills/<name>/SKILL.md (current Claude Code Skills format)")
	install.Flags().Bool("claude-code", false, "Install into .claude/commands/ as slash commands and write .claude/agents.md")
	install.Flags().Bool("codex", false, "Install into .codex/skills/")
	install.Flags().StringP("dir", "d", "", "Install into a custom directory path")

	onboard := cobra.Command{
		Use:   "onboard",
		Short: "Print all APImetrics skills to stdout for agent onboarding",
		Long: "Prints all embedded skill workflows to stdout.\n" +
			"Run this command and include the output in your agent's context\n" +
			"to onboard it on the correct APImetrics CLI workflows.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return printSkills()
		},
	}

	skills.AddCommand(&install)
	cmd.AddCommand(&skills)
	cmd.AddCommand(&onboard)
}

func resolveTarget(claudeCode, claudeSkills, codex bool, dir string) (string, agent, error) {
	count := 0
	if claudeCode {
		count++
	}
	if claudeSkills {
		count++
	}
	if codex {
		count++
	}
	if dir != "" {
		count++
	}

	if count == 0 {
		return "", agentCustom, fmt.Errorf("specify a target: --claude-skills, --claude-code, --codex, or --dir <path>")
	}
	if count > 1 {
		return "", agentCustom, fmt.Errorf("specify only one of --claude-skills, --claude-code, --codex, or --dir")
	}

	switch {
	case claudeSkills:
		return ".claude/skills", agentClaudeSkills, nil
	case claudeCode:
		return ".claude/commands", agentClaudeCode, nil
	case codex:
		return ".codex/skills", agentCodex, nil
	default:
		return dir, agentCustom, nil
	}
}

// installClaudeSkills writes the current-format Claude Code skills as
// <target>/<name>/SKILL.md, one directory per skill.
func installClaudeSkills(target string) error {
	if err := os.MkdirAll(target, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", target, err)
	}

	var names []string
	err := fs.WalkDir(claudeSkillFiles, "claude-skills", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || path.Base(p) != "SKILL.md" {
			return nil
		}

		data, err := claudeSkillFiles.ReadFile(p)
		if err != nil {
			return fmt.Errorf("reading skill %s: %w", p, err)
		}

		slug := path.Base(path.Dir(p))
		dir := filepath.Join(target, slug)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}

		dest := filepath.Join(dir, "SKILL.md")
		if err := os.WriteFile(dest, data, 0644); err != nil {
			return fmt.Errorf("writing skill %s: %w", dest, err)
		}

		fmt.Fprintf(cli.Stdout, "Installed %s\n", dest)
		names = append(names, slug)
		return nil
	})
	if err != nil {
		return fmt.Errorf("reading embedded skills: %w", err)
	}

	fmt.Fprintf(cli.Stdout, "\n%d skills installed to %s\n", len(names), target)
	fmt.Fprintf(cli.Stdout, "\nClaude Code loads these automatically by description, or you can invoke one by name:\n")
	for _, name := range names {
		fmt.Fprintf(cli.Stdout, "  /%s\n", name)
	}
	fmt.Fprintf(cli.Stdout, "\nTo onboard any agent directly, run: %s onboard\n", os.Args[0])
	return nil
}

func installSkills(target string, kind agent) error {
	if kind == agentClaudeSkills {
		return installClaudeSkills(target)
	}

	if err := os.MkdirAll(target, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", target, err)
	}

	entries, err := fs.ReadDir(skillFiles, "embed")
	if err != nil {
		return fmt.Errorf("reading embedded skills: %w", err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		data, err := skillFiles.ReadFile("embed/" + entry.Name())
		if err != nil {
			return fmt.Errorf("reading skill %s: %w", entry.Name(), err)
		}

		dest := filepath.Join(target, entry.Name())
		if err := os.WriteFile(dest, data, 0644); err != nil {
			return fmt.Errorf("writing skill %s: %w", dest, err)
		}

		fmt.Fprintf(cli.Stdout, "Installed %s\n", dest)
		names = append(names, strings.TrimSuffix(entry.Name(), ".md"))
	}

	fmt.Fprintf(cli.Stdout, "\nSkills installed to %s\n", target)

	switch kind {
	case agentClaudeCode:
		fmt.Fprintf(cli.Stdout, "\nAvailable as slash commands in Claude Code:\n")
		for _, name := range names {
			fmt.Fprintf(cli.Stdout, "  /%s\n", name)
		}
		if err := writeAgentsMD(names, os.Args[0]); err != nil {
			return err
		}
		if err := updateClaudeMD(); err != nil {
			return err
		}
	case agentCodex:
		fmt.Fprintf(cli.Stdout, "\nReference these files from your agent's context configuration.\n")
	}

	fmt.Fprintf(cli.Stdout, "\nTo onboard any agent directly, run: %s onboard\n", os.Args[0])
	return nil
}

func writeAgentsMD(skillNames []string, bin string) error {
	const path = ".claude/agents.md"

	content := buildAgentsMD(skillNames, bin)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	fmt.Fprintf(cli.Stdout, "\nWritten %s\n", path)
	return nil
}

// updateClaudeMD appends an @.claude/agents.md reference to CLAUDE.md if not already present.
func updateClaudeMD() error {
	const path = "CLAUDE.md"
	const ref = "@.claude/agents.md"

	existing, _ := os.ReadFile(path)

	if bytes.Contains(existing, []byte(ref)) {
		return nil
	}

	updated := existing
	if len(updated) > 0 && updated[len(updated)-1] != '\n' {
		updated = append(updated, '\n')
	}
	if len(updated) > 0 {
		updated = append(updated, '\n')
	}
	updated = append(updated, []byte(ref+"\n")...)

	if err := os.WriteFile(path, updated, 0644); err != nil {
		return fmt.Errorf("updating CLAUDE.md: %w", err)
	}

	fmt.Fprintf(cli.Stdout, "Added @.claude/agents.md reference to CLAUDE.md\n")
	return nil
}

func buildAgentsMD(skillNames []string, bin string) string {
	var b strings.Builder

	b.WriteString("# APImetrics CLI Agent Guide\n\n")
	b.WriteString(fmt.Sprintf("Use `%s` for all apimetrics commands in this project.\n\n", bin))

	b.WriteString("## Prerequisites\n\n")
	b.WriteString("Before running any command:\n\n")
	b.WriteString(fmt.Sprintf("1. Run `%s project show` to confirm a project is active — all commands fail without one. If none is set, run `%s project select`.\n", bin, bin))
	b.WriteString("2. Invoke the relevant skill for the task.\n\n")

	b.WriteString("## Critical facts\n\n")
	b.WriteString("- Commands are flat, not grouped: use `create-call`, not `calls create`\n")
	b.WriteString("- All create commands read JSON from stdin using heredoc syntax — there is no `--body`, `--data`, or `-d` flag\n")
	b.WriteString(fmt.Sprintf("- Correct pattern: `%s create-call <<'EOF' ... EOF`\n\n", bin))

	b.WriteString("## Skills\n\n")
	for _, name := range skillNames {
		b.WriteString(fmt.Sprintf("- `/%s`\n", name))
	}
	b.WriteString(fmt.Sprintf("\nIf slash commands are unavailable or you need to refresh context, run:\n\n```bash\n%s onboard\n```\n\nThis works for any agent and always reflects the current skill set.\n", bin))

	return b.String()
}

func printSkills() error {
	entries, err := fs.ReadDir(skillFiles, "embed")
	if err != nil {
		return fmt.Errorf("reading embedded skills: %w", err)
	}

	first := true
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		data, err := skillFiles.ReadFile("embed/" + entry.Name())
		if err != nil {
			return fmt.Errorf("reading skill %s: %w", entry.Name(), err)
		}

		if !first {
			fmt.Fprintln(cli.Stdout)
		}
		first = false

		fmt.Fprint(cli.Stdout, string(data))
	}

	return nil
}
