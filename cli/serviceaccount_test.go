package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeServiceAccount writes contents to a temp file and returns its path.
func writeServiceAccount(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "service-account.json")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0600))
	return path
}

func TestLoadServiceAccount(t *testing.T) {
	buildCfg.TokenURL = "https://auth.example.com/oauth/token"
	buildCfg.AuthAudience = "https://api.example.com"

	path := writeServiceAccount(t, `{
		"name": "ci-runner",
		"client_id": "abc123",
		"client_secret": "s3cret",
		"audience": "https://client.apimetrics.io"
	}`)

	sa, err := loadServiceAccount(path)
	require.NoError(t, err)

	assert.Equal(t, "ci-runner", sa.Name)
	assert.Equal(t, "abc123", sa.ClientID)
	assert.Equal(t, "s3cret", sa.ClientSecret)
	assert.Equal(t, "https://client.apimetrics.io", sa.Audience)

	// Files issued today carry no token URL, so the build-time one is used.
	assert.Equal(t, "https://auth.example.com/oauth/token", sa.TokenURL)
}

func TestLoadServiceAccountDefaults(t *testing.T) {
	buildCfg.TokenURL = "https://auth.example.com/oauth/token"
	buildCfg.AuthAudience = "https://api.example.com"

	path := writeServiceAccount(t, `{"client_id": "abc123", "client_secret": "s3cret"}`)

	sa, err := loadServiceAccount(path)
	require.NoError(t, err)

	assert.Equal(t, "https://api.example.com", sa.Audience)
	assert.Equal(t, "https://auth.example.com/oauth/token", sa.TokenURL)
}

func TestLoadServiceAccountOverridesTokenURL(t *testing.T) {
	buildCfg.TokenURL = "https://auth.example.com/oauth/token"

	path := writeServiceAccount(t, `{
		"client_id": "abc123",
		"client_secret": "s3cret",
		"token_url": "https://other.example.com/oauth/token"
	}`)

	sa, err := loadServiceAccount(path)
	require.NoError(t, err)
	assert.Equal(t, "https://other.example.com/oauth/token", sa.TokenURL)
}

func TestLoadServiceAccountErrors(t *testing.T) {
	_, err := loadServiceAccount(filepath.Join(t.TempDir(), "nope.json"))
	assert.ErrorIs(t, err, os.ErrNotExist)

	_, err = loadServiceAccount(writeServiceAccount(t, "not json"))
	assert.ErrorContains(t, err, "not a valid service account file")

	_, err = loadServiceAccount(writeServiceAccount(t, `{"name": "incomplete"}`))
	assert.ErrorContains(t, err, "client_id, client_secret")

	_, err = loadServiceAccount(writeServiceAccount(t, `{"client_id": "abc123"}`))
	assert.ErrorContains(t, err, "missing required field(s): client_secret")
}

func TestServiceAccountCacheKey(t *testing.T) {
	sa := &ServiceAccount{ClientID: "abc123", ClientSecret: "s3cret", TokenURL: "https://auth.example.com/oauth/token"}

	// Stable across calls, and namespaced away from the interactive login's
	// "apimetrics:<profile>" key.
	assert.Equal(t, sa.cacheKey(), sa.cacheKey())
	assert.Regexp(t, `^service-account:[0-9a-f]{16}$`, sa.cacheKey())

	// Every credential field takes part, so no two accounts — and no rotated
	// secret — can be served another's cached token.
	other := *sa
	other.ClientID = "different"
	assert.NotEqual(t, sa.cacheKey(), other.cacheKey())

	rotated := *sa
	rotated.ClientSecret = "rotated"
	assert.NotEqual(t, sa.cacheKey(), rotated.cacheKey())

	otherEnv := *sa
	otherEnv.TokenURL = "https://qc-auth.example.com/oauth/token"
	assert.NotEqual(t, sa.cacheKey(), otherEnv.cacheKey())
}

func TestApplyCredentialOverrides(t *testing.T) {
	buildCfg.TokenURL = "https://auth.example.com/oauth/token"
	buildCfg.AuthAudience = "https://api.example.com"

	defer func(sa *ServiceAccount, project string) {
		activeServiceAccount, projectIDOverride = sa, project
	}(activeServiceAccount, projectIDOverride)

	initAPIConfig()
	profile := configs["apimetrics"].Profiles["default"]
	assert.Equal(t, "oauth-authorization-code", profile.Auth.Name)

	activeServiceAccount = &ServiceAccount{
		ClientID:     "abc123",
		ClientSecret: "s3cret",
		Audience:     "https://api.example.com",
		TokenURL:     buildCfg.TokenURL,
	}
	projectIDOverride = "project-42"
	applyCredentialOverrides()

	assert.Equal(t, "oauth-service-account", profile.Auth.Name)
	assert.Equal(t, "abc123", profile.Auth.Params["client_id"])
	assert.Equal(t, "s3cret", profile.Auth.Params["client_secret"])
	assert.Equal(t, buildCfg.TokenURL, profile.Auth.Params["token_url"])
	assert.Equal(t, "https://api.example.com", profile.Auth.Params["audience"])
	assert.Equal(t, activeServiceAccount.cacheKey(), profile.Auth.Params["cache_key"])
	assert.Equal(t, "project-42", profile.Headers["Apimetrics-Project-Id"])
}
