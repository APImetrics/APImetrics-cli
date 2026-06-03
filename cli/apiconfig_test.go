package cli

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEditAPIsMissingEditor(t *testing.T) {
	os.Setenv("EDITOR", "")
	os.Setenv("VISUAL", "")
	exited := false
	editAPIs(func(code int) {
		exited = true
	})
	assert.True(t, exited)
}

func TestEditBadCommand(t *testing.T) {
	os.Setenv("EDITOR", "bad-command")
	os.Setenv("VISUAL", "")
	assert.Panics(t, func() {
		editAPIs(func(code int) {})
	})
}
