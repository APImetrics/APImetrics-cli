package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/google/shlex"
	"github.com/spf13/viper"
)

// buildCfg holds the API endpoint and auth parameters baked in at build time.
var buildCfg struct {
	Environment  string
	BaseURL      string
	AuthURL      string
	TokenURL     string
	AuthAudience string
	ClientID     string
}

// SetBuildConfig sets the API endpoint and auth parameters baked in at build
// time. Must be called before cli.Init.
func SetBuildConfig(baseURL, authURL, tokenURL, authAudience, clientID string) {
	buildCfg.BaseURL = baseURL
	buildCfg.AuthURL = authURL
	buildCfg.TokenURL = tokenURL
	buildCfg.AuthAudience = authAudience
	buildCfg.ClientID = clientID
}

// SetEnvironment records which APImetrics environment this binary was built
// against (e.g. "production", "qc"). Shown by `--version` so parallel installs
// can be told apart. Must be called before cli.Init.
func SetEnvironment(env string) {
	buildCfg.Environment = env
}

// APIAuth describes the auth type and parameters for an API.
type APIAuth struct {
	Name   string            `json:"name" yaml:"name"`
	Params map[string]string `json:"params,omitempty" yaml:"params,omitempty"`
}

// TLSConfig contains the TLS setup for the HTTP client
type TLSConfig struct {
	InsecureSkipVerify bool          `json:"insecure,omitempty" yaml:"insecure,omitempty" mapstructure:"insecure"`
	Cert               string        `json:"cert,omitempty" yaml:"cert,omitempty"`
	Key                string        `json:"key,omitempty" yaml:"key,omitempty"`
	CACert             string        `json:"ca_cert,omitempty" yaml:"ca_cert,omitempty" mapstructure:"ca_cert"`
	PKCS11             *PKCS11Config `json:"pkcs11,omitempty" yaml:"pkcs11,omitempty"`
}

// PKCS11Config contains information about how to get a client certificate
// from a hardware device via PKCS#11.
type PKCS11Config struct {
	Path  string `json:"path,omitempty" yaml:"path,omitempty"`
	Label string `json:"label" yaml:"label"`
}

// APIProfile contains account-specific API information
type APIProfile struct {
	Base    string            `json:"base,omitempty" yaml:"base,omitempty"`
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Query   map[string]string `json:"query,omitempty" yaml:"query,omitempty"`
	Auth    *APIAuth          `json:"auth,omitempty" yaml:"auth,omitempty"`
}

// APIConfig describes per-API configuration options like the base URI and
// auth scheme, if any.
type APIConfig struct {
	name          string
	Base          string                 `json:"base" yaml:"base"`
	OperationBase string                 `json:"operation_base,omitempty" yaml:"operation_base,omitempty" mapstructure:"operation_base,omitempty"`
	SpecFiles     []string               `json:"spec_files,omitempty" yaml:"spec_files,omitempty" mapstructure:"spec_files,omitempty"`
	Profiles      map[string]*APIProfile `json:"profiles,omitempty" yaml:"profiles,omitempty" mapstructure:",omitempty"`
	TLS           *TLSConfig             `json:"tls,omitempty" yaml:"tls,omitempty" mapstructure:",omitempty"`
}

// Save is a no-op: API configuration is static, baked in at build time.
func (a APIConfig) Save() error { return nil }

type apiConfigs map[string]*APIConfig

var configs apiConfigs

func initAPIConfig() {
	auth := &APIAuth{
		Name: "oauth-authorization-code",
		Params: map[string]string{
			"authorize_url": buildCfg.AuthURL + "?audience=" + buildCfg.AuthAudience,
			"token_url":     buildCfg.TokenURL,
			"client_id":     buildCfg.ClientID,
			"client_secret": "",
			"redirect_url":  "",
			"scopes":        "openid profile email",
		},
	}

	profile := &APIProfile{Auth: auth}
	if projectID := activeProjectID(); projectID != "" {
		profile.Headers = map[string]string{
			"Apimetrics-Project-Id": projectID,
		}
	}

	configs = apiConfigs{
		"apimetrics": {
			name: "apimetrics",
			Base: buildCfg.BaseURL,
			Profiles: map[string]*APIProfile{
				"default": profile,
			},
		},
	}
}

// applyCredentialOverrides re-points the default profile at a service account
// and at any project chosen for this run. The command line is not parsed until
// Run, so this happens there rather than in initAPIConfig, but still before
// anything hits the wire.
func applyCredentialOverrides() {
	cfg := configs["apimetrics"]
	if cfg == nil {
		return
	}

	profile := cfg.Profiles["default"]
	if profile == nil {
		return
	}

	// A service account replaces the browser login entirely.
	if sa := activeServiceAccount; sa != nil {
		profile.Auth = &APIAuth{
			Name: "oauth-service-account",
			Params: map[string]string{
				"client_id":     sa.ClientID,
				"client_secret": sa.ClientSecret,
				"token_url":     sa.TokenURL,
				"audience":      sa.Audience,
				"cache_key":     sa.cacheKey(),
			},
		}
	}

	if projectID := activeProjectID(); projectID != "" {
		if profile.Headers == nil {
			profile.Headers = map[string]string{}
		}
		profile.Headers["Apimetrics-Project-Id"] = projectID
	}
}

func findAPI(uri string) (string, *APIConfig) {
	apiName := viper.GetString("api-name")

	for name, config := range configs {
		// fixes https://apicontext.com/apimetrics/issues/128
		if len(apiName) > 0 && name != apiName {
			continue
		}

		profile := viper.GetString("rsh-profile")
		if profile != "default" {
			if config.Profiles[profile] == nil {
				continue
			}
			if config.Profiles[profile].Base != "" {
				if strings.HasPrefix(uri, config.Profiles[profile].Base) {
					return name, config
				}
			} else if config.Base != "" && strings.HasPrefix(uri, config.Base) {
				return name, config
			}
		} else {
			if config.Base != "" && strings.HasPrefix(uri, config.Base) {
				// TODO: find the longest matching base?
				return name, config
			}
		}
	}

	return "", nil
}

func editAPIs(exitFunc func(int)) {
	editor := getEditor()
	if editor == "" {
		fmt.Fprintln(os.Stderr, `Please set the VISUAL or EDITOR environment variable with your preferred editor. Examples:

export VISUAL="code --wait"
export EDITOR="vim"`)
		exitFunc(1)
		return
	}

	parts, err := shlex.Split(editor)
	panicOnErr(err)
	name := parts[0]
	args := append(parts[1:], path.Join(
		getConfigDir(viper.GetString("app-name")), "apis.json",
	))

	c := exec.Command(name, args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	panicOnErr(c.Run())
}
