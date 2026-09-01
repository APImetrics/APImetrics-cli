package skills

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveTarget(t *testing.T) {
	cases := []struct {
		name       string
		claudeCode bool
		codex      bool
		dir        string
		wantTarget string
		wantAgent  agent
		wantErr    string
	}{
		{
			name:    "no target selected",
			wantErr: "specify a target: --claude-code, --codex, or --dir <path>",
		},
		{
			name:       "claude-code and codex together",
			claudeCode: true,
			codex:      true,
			wantErr:    "specify only one of --claude-code, --codex, or --dir",
		},
		{
			name:       "claude-code and dir together",
			claudeCode: true,
			dir:        "some/dir",
			wantErr:    "specify only one of --claude-code, --codex, or --dir",
		},
		{
			name:    "codex and dir together",
			codex:   true,
			dir:     "some/dir",
			wantErr: "specify only one of --claude-code, --codex, or --dir",
		},
		{
			name:       "all three together",
			claudeCode: true,
			codex:      true,
			dir:        "some/dir",
			wantErr:    "specify only one of --claude-code, --codex, or --dir",
		},
		{
			name:       "claude-code only",
			claudeCode: true,
			wantTarget: ".claude/commands",
			wantAgent:  agentClaudeCode,
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
			target, kind, err := resolveTarget(c.claudeCode, c.codex, c.dir)

			if c.wantErr != "" {
				assert.EqualError(t, err, c.wantErr)
				assert.Empty(t, target)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, c.wantTarget, target)
			assert.Equal(t, c.wantAgent, kind)
		})
	}
}
