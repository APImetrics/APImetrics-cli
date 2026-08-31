package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ServiceAccount holds the credentials from an APImetrics service account
// file, as downloaded from the APImetrics web app when the service account is
// created. Pointing the CLI at one of these files authenticates every request
// with the OAuth 2.0 client credentials grant instead of a browser login,
// which is what makes the CLI usable from CI and other headless environments.
type ServiceAccount struct {
	Name         string `json:"name"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Audience     string `json:"audience"`

	// TokenURL is not written into the files APImetrics issues today. It is
	// honoured when present so a file can point at a non-default auth service;
	// otherwise the URL baked in at build time is used.
	TokenURL string `json:"token_url,omitempty"`
}

// activeServiceAccount is the service account this run authenticates with, or
// nil when authenticating interactively.
var activeServiceAccount *ServiceAccount

// loadServiceAccount reads and validates a service account file, filling in
// the build-time defaults for anything the file doesn't specify.
func loadServiceAccount(path string) (*ServiceAccount, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	sa := &ServiceAccount{}
	if err := json.Unmarshal(b, sa); err != nil {
		return nil, fmt.Errorf("%s is not a valid service account file: %w", path, err)
	}

	missing := []string{}
	if sa.ClientID == "" {
		missing = append(missing, "client_id")
	}
	if sa.ClientSecret == "" {
		missing = append(missing, "client_secret")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("%s is missing required field(s): %s", path, strings.Join(missing, ", "))
	}

	if sa.Audience == "" {
		sa.Audience = buildCfg.AuthAudience
	}
	if sa.TokenURL == "" {
		sa.TokenURL = buildCfg.TokenURL
	}

	return sa, nil
}

// cacheKey returns the cache key for this service account's access token.
// Tokens are keyed by credential rather than by profile so that switching
// between service accounts — or between a service account and an interactive
// login — never reuses another identity's token. The secret is part of the key
// so that rotating it doesn't leave the old token being served from cache.
func (sa *ServiceAccount) cacheKey() string {
	sum := sha256.Sum256([]byte(sa.ClientID + "|" + sa.ClientSecret + "|" + sa.TokenURL))
	return "service-account:" + hex.EncodeToString(sum[:8])
}

// initServiceAccount resolves the `--service-account` flag or the
// `<APP_NAME>_SERVICE_ACCOUNT` environment variable and loads the file it
// names. Called from Run once the global flags are parsed, before
// applyCredentialOverrides builds the auth profile around the result.
//
// A missing or malformed file is fatal. Quietly falling back to a browser
// login when the user asked for a service account would be surprising
// interactively, and would hang in the headless environments this is for.
func initServiceAccount(appName string) {
	path, _ := GlobalFlags.GetString("service-account")
	if path == "" {
		path = os.Getenv(envVarName(appName, "_SERVICE_ACCOUNT"))
	}
	if path == "" {
		return
	}

	sa, err := loadServiceAccount(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not load service account: %v\n", err)
		os.Exit(1)
	}

	activeServiceAccount = sa
}
