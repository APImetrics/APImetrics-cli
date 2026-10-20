package skills

import (
	"bytes"
	"io/fs"
	"path"
	"strings"
	"testing"

	"apicontext.com/apimetrics/cli"

	"github.com/stretchr/testify/assert"
)

func TestResolveTarget(t *testing.T) {
	cases := []struct {
		name         string
		claudeCode   bool
		claudeSkills bool
		codex        bool
		dir          string
		wantTarget   string
		wantAgent    agent
		wantErr      string
	}{
		{
			name:    "no target selected",
			wantErr: "specify a target: --claude-skills, --claude-code, --codex, or --dir <path>",
		},
		{
			name:       "claude-code and codex together",
			claudeCode: true,
			codex:      true,
			wantErr:    "specify only one of --claude-skills, --claude-code, --codex, or --dir",
		},
		{
			name:       "claude-code and dir together",
			claudeCode: true,
			dir:        "some/dir",
			wantErr:    "specify only one of --claude-skills, --claude-code, --codex, or --dir",
		},
		{
			name:    "codex and dir together",
			codex:   true,
			dir:     "some/dir",
			wantErr: "specify only one of --claude-skills, --claude-code, --codex, or --dir",
		},
		{
			name:       "all three together",
			claudeCode: true,
			codex:      true,
			dir:        "some/dir",
			wantErr:    "specify only one of --claude-skills, --claude-code, --codex, or --dir",
		},
		{
			name:         "claude-skills and claude-code together",
			claudeSkills: true,
			claudeCode:   true,
			wantErr:      "specify only one of --claude-skills, --claude-code, --codex, or --dir",
		},
		{
			name:       "claude-code only",
			claudeCode: true,
			wantTarget: ".claude/commands",
			wantAgent:  agentClaudeCode,
		},
		{
			name:         "claude-skills only",
			claudeSkills: true,
			wantTarget:   ".claude/skills",
			wantAgent:    agentClaudeSkills,
		},
		{
			name:       "codex only",
			codex:      true,
			wantTarget: ".codex/skills",
			wantAgent:  agentCodex,
		},
		{
			name:       "dir only",
			dir:        "some/dir",
			wantTarget: "some/dir",
			wantAgent:  agentCustom,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			target, kind, err := resolveTarget(c.claudeCode, c.claudeSkills, c.codex, c.dir)

			if c.wantErr != "" {
				assert.EqualError(t, err, c.wantErr)
				assert.Empty(t, target)
				assert.Equal(t, agentCustom, kind)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, c.wantTarget, target)
			assert.Equal(t, c.wantAgent, kind)
		})
	}
}

func TestPrintSkillsIncludesEveryWorkflow(t *testing.T) {
	var buf bytes.Buffer
	orig := cli.Stdout
	cli.Stdout = &buf
	defer func() { cli.Stdout = orig }()

	assert.NoError(t, printSkills())
	out := buf.String()

	var count int
	err := fs.WalkDir(claudeSkillFiles, "claude-skills", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Base(p) != "SKILL.md" {
			return err
		}
		count++
		data, err := claudeSkillFiles.ReadFile(p)
		if err != nil {
			return err
		}
		assert.True(t, strings.Contains(out, string(data)), "onboard output is missing %s", p)
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 8, count)

	entries, err := fs.ReadDir(skillFiles, "embed")
	assert.NoError(t, err)
	for _, e := range entries {
		data, err := skillFiles.ReadFile("embed/" + e.Name())
		assert.NoError(t, err)
		assert.True(t, strings.Contains(out, string(data)), "onboard output is missing embed/%s", e.Name())
	}
}
