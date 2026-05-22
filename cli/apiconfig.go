package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/shlex"
	"github.com/spf13/viper"
)

// apis holds the per-API configuration.
var apis *viper.Viper

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

// Save the API configuration to disk.
func (a APIConfig) Save() error {
	apis.Set(a.name, a)
	return apis.WriteConfig()
}

// Return colorized string of configuration in JSON or YAML
func (a APIConfig) GetPrettyDisplay(outFormat string) ([]byte, error) {
	// marshal
	if outFormat == "auto" {
		outFormat = "json"
	}
	marshalled, err := MarshalShort(outFormat, true, a)
	if err != nil {
		return nil, errors.New("unable to render configuration")
	}

	if !useColor {
		return marshalled, nil
	}

	// colorize
	marshalled, err = Highlight(outFormat, marshalled)
	if err != nil {
		return nil, errors.New("unable to colorize output")
	}

	return marshalled, nil
}

type apiConfigs map[string]*APIConfig

var configs apiConfigs

func initAPIConfig() {
	apis = viper.New()

	apis.SetConfigName("apis")
	apis.AddConfigPath(viper.GetString("config-directory"))

	// Write a blank cache if no file is already there. Later you can use
	// configs.SaveConfig() to write new values.
	filename := filepath.Join(viper.GetString("config-directory"), "apis.json")
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		if err := os.WriteFile(filename, []byte("{}"), 0600); err != nil {
			panic(err)
		}
	}

	err := apis.ReadInConfig()
	if err != nil {
		panic(err)
	}

	if apis.GetString("$schema") == "" {
		apis.Set("$schema", "https://schemas.apicontext.com/cli/apis.json")
		apis.WriteConfig()
	}

	// Register API sub-commands
	configs = apiConfigs{}
	tmp := viper.New()
	for k, v := range apis.AllSettings() {
		if k == "$schema" {
			continue
		}
		tmp.Set(k, v)
	}
	if err := tmp.Unmarshal(&configs); err != nil {
		panic(err)
	}

	seen := map[string]bool{}
	for apiName, config := range configs {
		if seen[config.Base] {
			panic(fmt.Errorf("multiple APIs configured with the same base URL: %s", config.Base))
		}
		seen[config.Base] = true
		config.name = apiName
		configs[apiName] = config
	}
}

func findAPI(uri string) (string, *APIConfig) {
	apiName := viper.GetString("api-name")

	for name, config := range configs {
		// fixes https://github.com/rest-sh/restish/issues/128
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
			} else if strings.HasPrefix(uri, config.Base) {
				return name, config
			}
		} else {
			if strings.HasPrefix(uri, config.Base) {
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
