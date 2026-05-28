package skills

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"apicontext.com/apimetrics/cli"
	"github.com/spf13/cobra"
)

//go:embed embed/*.md
var skillFiles embed.FS

// Init registers the skills command group on the parent command.
func Init(cmd *cobra.Command) {
	skills := cobra.Command{
		Use:   "skills",
		Short: "Manage agent skills for APImetrics",
	}

	install := cobra.Command{
		Use:   "install",
		Short: "Install APImetrics skills into an agent skills directory",
		Example: "  " + os.Args[0] + " skills install --claude-code\n" +
			"  " + os.Args[0] + " skills install --codex\n" +
			"  " + os.Args[0] + " skills install --dir ./custom",
		RunE: func(cmd *cobra.Command, args []string) error {
			claudeCode, _ := cmd.Flags().GetBool("claude-code")
			codex, _ := cmd.Flags().GetBool("codex")
			dir, _ := cmd.Flags().GetString("dir")

			target, err := resolveTarget(claudeCode, codex, dir)
			if err != nil {
				return err
			}

			return installSkills(target)
		},
	}

	install.Flags().Bool("claude-code", false, "Install into .claude/skills/")
	install.Flags().Bool("codex", false, "Install into .codex/skills/")
	install.Flags().StringP("dir", "d", "", "Install into a custom directory path")

	skills.AddCommand(&install)
	cmd.AddCommand(&skills)
}

func resolveTarget(claudeCode, codex bool, dir string) (string, error) {
	count := 0
	if claudeCode {
		count++
	}
	if codex {
		count++
	}
	if dir != "" {
		count++
	}

	if count == 0 {
		return "", fmt.Errorf("specify a target: --claude-code, --codex, or --dir <path>")
	}
	if count > 1 {
		return "", fmt.Errorf("specify only one of --claude-code, --codex, or --dir")
	}

	switch {
	case claudeCode:
		return ".claude/skills", nil
	case codex:
		return ".codex/skills", nil
	default:
		return dir, nil
	}
}

func installSkills(target string) error {
	if err := os.MkdirAll(target, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", target, err)
	}

	entries, err := fs.ReadDir(skillFiles, "embed")
	if err != nil {
		return fmt.Errorf("reading embedded skills: %w", err)
	}

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
	}

	fmt.Fprintf(cli.Stdout, "\nSkills installed to %s\n", target)
	return nil
}
