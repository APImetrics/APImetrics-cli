package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/logrusorgru/aurora"
	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

// Root command (entrypoint) of the CLI.
var Root *cobra.Command

// GlobalFlags contains all the fixed up front flags
// This allows us to parse them before we hand over control
// to cobra
var GlobalFlags *pflag.FlagSet

// Cache is used to store temporary data between runs.
var Cache *viper.Viper

// Formatter is the currently configured response output formatter.
var Formatter ResponseFormatter

// Stdout is a cross-platform, color-safe writer if colors are enabled,
// otherwise it defaults to `os.Stdout`.
var Stdout io.Writer = os.Stdout

// Stderr is a cross-platform, color-safe writer if colors are enabled,
// otherwise it defaults to `os.Stderr`.
var Stderr io.Writer = os.Stderr

var useColor bool
var au aurora.Aurora

// Keeps track of currently selected API for shell completions
var currentConfig *APIConfig

// Init will set up the CLI.
func Init(name string, version string) {
	initConfig(name, "")
	initCache(name)

	// Reset registries.
	authHandlers = map[string]AuthHandler{}
	contentTypes = map[string]contentTypeEntry{}
	encodings = map[string]ContentEncoding{}
	linkParsers = []LinkParser{}
	loaders = []Loader{}

	// Determine if we are using a TTY or colored output is forced-on.
	tty := false
	if isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd()) || viper.GetBool("tty") {
		tty = true
		viper.Set("tty", true)
	}

	useColor = false
	if viper.GetBool("color") || (tty && !viper.GetBool("nocolor")) {
		useColor = true
	}

	if useColor {
		// Support colored output across operating systems.
		Stdout = colorable.NewColorableStdout()
		Stderr = colorable.NewColorableStderr()

		viper.Set("color", useColor)
	}

	au = aurora.NewAurora(useColor)

	Formatter = NewDefaultFormatter(tty, useColor)

	cobra.AddTemplateFunc("versionExtra", versionExtraInfo)

	cobra.AddTemplateFunc("highlight", func(s string) string {
		// Highlighting is expensive, so only do this when the user actually asks
		// for help via this template func and a custom help template.
		if tty {
			w, _, err := term.GetSize(0)
			if err != nil {
				// Default to standard terminal size
				w = 80
			}
			r, _ := glamour.NewTermRenderer(
				glamour.WithStyles(MarkdownStyle),
				glamour.WithWordWrap(w),
			)
			if out, err := r.Render(s); err == nil {
				return out
			}
		}
		return s
	})

	Root = &cobra.Command{
		Use:     filepath.Base(os.Args[0]),
		Long:    "The APImetrics CLI — manage monitors, SLOs, schedules, and more from your terminal.",
		Version: version,
		Example: fmt.Sprintf(`  # List your monitors
  $ %s list-monitors

  # Create a monitor
  $ %s create-monitor name: "Checkout flow", url: https://example.com/checkout`, name, name),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			settings := viper.AllSettings()
			LogDebug("Configuration: %v", settings)
			return ensureProject(cmd)
		},
	}
	Root.SetHelpTemplate(`{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces | highlight}}

{{end}}{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`)
	Root.SetVersionTemplate(`{{with .Name}}{{printf "%s " .}}{{end}}{{printf "version %s" .Version}}
{{versionExtra}}
`)

	GlobalFlags = pflag.NewFlagSet("eager-flags", pflag.ContinueOnError)
	GlobalFlags.ParseErrorsWhitelist.UnknownFlags = true
	// GlobalFlags are 'hidden', don't print anything on error
	GlobalFlags.Usage = func() {}
	// Ensure parsing doesn't stop if the help flag is set
	// (help seems to be special cased from ParseErrorsWhitelist.UnknownFlags)
	GlobalFlags.BoolP("help", "h", false, "")
	// Mirror Cobra's auto-registered version flag here so we can detect it
	// during the eager parse below, before the command tree is loaded. This
	// handles every bool form (`--version`, `--version=true`, `--version=false`).
	GlobalFlags.Bool("version", false, "")

	AddGlobalFlag("rsh-verbose", "v", "Enable verbose log output", false, false)
	AddGlobalFlag("rsh-output-format", "o", "Output format [auto, json, table, ...]", "auto", false)
	AddGlobalFlag("rsh-filter", "f", "Filter / project results using shorthand query", "", false)
	AddGlobalFlag("rsh-raw", "r", "Output result of query as raw rather than an escaped JSON string or list", false, false)
	AddGlobalFlag("rsh-server", "s", "Override scheme://server:port for an API", "", false)
	AddGlobalFlag("rsh-header", "H", "Add custom header", []string{}, true)
	AddGlobalFlag("rsh-query", "q", "Add custom query param", []string{}, true)
	AddGlobalFlag("rsh-no-paginate", "", "Disable auto-pagination", false, false)
	AddGlobalFlag("rsh-profile", "p", "API auth profile", "default", false)
	AddGlobalFlag("rsh-no-cache", "", "Disable HTTP cache", false, false)
	AddGlobalFlag("rsh-insecure", "", "Disable SSL verification", false, false)
	AddGlobalFlag("rsh-client-cert", "", "Path to a PEM encoded client certificate", "", false)
	AddGlobalFlag("rsh-client-key", "", "Path to a PEM encoded private key", "", false)
	AddGlobalFlag("rsh-ca-cert", "", "Path to a PEM encoded CA cert", "", false)
	AddGlobalFlag("rsh-ignore-status-code", "", "Do not set exit code from HTTP status code", false, false)
	AddGlobalFlag("rsh-retry", "", "Number of times to retry on certain failures", 2, false)
	AddGlobalFlag("rsh-timeout", "t", "Timeout for HTTP requests", time.Duration(0), false)
	AddGlobalFlag("service-account", "", "Path to a service account file to authenticate with instead of a browser login", "", false)
	AddGlobalFlag("project-id", "", "Project to use for this run, overriding the selected project", "", false)

	Root.RegisterFlagCompletionFunc("rsh-output-format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"auto", "json", "yaml"}, cobra.ShellCompDirectiveNoFileComp
	})

	Root.RegisterFlagCompletionFunc("rsh-profile", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		profiles := []string{}
		if currentConfig != nil {
			for profile := range currentConfig.Profiles {
				profiles = append(profiles, profile)
			}
		}
		return profiles, cobra.ShellCompDirectiveNoFileComp
	})

	initAPIConfig()
	AddConfigCommands(Root)
}

func userHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic("HOME directoy is not defined")
	}
	return home
}

// envVarName turns an app name into an environment variable prefix, e.g.
// "apimetrics-qc" -> "APIMETRICS_QC". Hyphens can't appear in a portable shell
// variable name, so they become underscores.
func envVarName(appName, suffix string) string {
	return strings.ToUpper(strings.ReplaceAll(appName, "-", "_")) + suffix
}

func getConfigDir(appName string) string {
	configDirEnv := envVarName(appName, "_CONFIG_DIR")
	configDir := os.Getenv(configDirEnv)

	if configDir == "" {
		// Create new config directory
		configBase, _ := os.UserConfigDir()
		configDir = filepath.Join(configBase, appName)

		// Check for legacy config dir
		legacyConfigDir := filepath.Join(viper.GetString("home-directory"), "."+appName)
		if _, err := os.Stat(legacyConfigDir); err == nil {
			// Only migrate if the new config dir doesn't exist, so that this
			// is a one-time operation. There are edge cases where configs could
			// get lost if we migrate every time (e.g. running an old version
			// that creates an empty ~/.restish/apis.json).
			if _, err := os.Stat(configDir); err != nil {
				// Define files to migrate
				for _, filename := range []string{
					"config.json",
					"apis.json",
					"cache.json",
				} {
					oldPath := filepath.Join(legacyConfigDir, filename)
					newDir := configDir
					if filename == "cache.json" {
						newDir = getCacheDir()
					}
					if _, err := os.Stat(oldPath); err == nil {
						os.MkdirAll(newDir, 0700)
						os.Rename(oldPath, filepath.Join(newDir, filename))
					}
				}
				// Everything else is a cache that can be regenerated
				os.RemoveAll(legacyConfigDir)
			}
		}
	}
	return configDir
}

func getCacheDir() string {
	appName := viper.GetString("app-name")
	cacheDirEnv := envVarName(appName, "_CACHE_DIR")

	cacheDir := os.Getenv(cacheDirEnv)

	if cacheDir == "" {
		cache, _ := os.UserCacheDir()
		cacheDir = filepath.Join(cache, appName)
	}
	return cacheDir
}

func initConfig(appName, envPrefix string) {
	viper.Set("app-name", appName)

	// One-time setup to ensure the path exists so we can write files into it
	// later as needed.
	home := userHomeDir()
	viper.Set("home-directory", home)

	configDir := getConfigDir(appName)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		panic(err)
	}

	// Load configuration from file(s) if provided.
	viper.SetConfigName("config")
	viper.AddConfigPath(filepath.Join("/etc/", appName))
	viper.AddConfigPath(filepath.Join(viper.GetString("home-directory"), "."+appName))
	viper.AddConfigPath(configDir)
	viper.ReadInConfig()

	// Load configuration from the environment if provided. Flags below get
	// transformed automatically, e.g. `client-id` -> `PREFIX_CLIENT_ID`.
	viper.SetEnvPrefix(envPrefix)
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	// Save a few things that will be useful elsewhere.
	viper.Set("config-directory", configDir)
	viper.SetDefault("server-index", 0)
}

func initCache(appName string) {
	Cache = viper.New()
	Cache.SetConfigName("cache")

	cacheDir := getCacheDir()
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		panic(err)
	}

	Cache.AddConfigPath(cacheDir)

	// Write a blank cache if no file is already there. Later you can use
	// cli.Cache.SaveConfig() to write new values.
	filename := filepath.Join(cacheDir, "cache.json")
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		if err := os.WriteFile(filename, []byte("{}"), 0600); err != nil {
			panic(err)
		}
	}
	viper.Set("cache-dir", cacheDir)
	Cache.ReadInConfig()
}

// Defaults adds the default encodings, content types, and link parsers to
// the CLI.
func Defaults() {
	// Register content encodings
	AddEncoding("deflate", &DeflateEncoding{})
	AddEncoding("gzip", &GzipEncoding{})
	AddEncoding("br", &BrotliEncoding{})

	// Register content type marshallers
	AddContentType("json", "application/json", 0.9, &JSON{})
	AddContentType("yaml", "application/yaml", 0.5, &YAML{})
	AddContentType("text", "text/*", 0.2, &Text{})
	AddContentType("table", "", -1, &Table{})
	AddContentType("readable", "", -1, &Readable{})
	AddContentType("gron", "", -1, &Gron{})

	// Add link relation parsers
	AddLinkParser(&LinkHeaderParser{})

}

// Run the CLI! Parse arguments, make requests, print responses.
func Run() (returnErr error) {
	if os.Getenv("COLOR") != "" {
		viper.Set("color", true)
	}
	if os.Getenv("NOCOLOR") != "" {
		viper.Set("nocolor", true)
	}

	// Because we may be doing HTTP calls before cobra has parsed the flags
	// we parse the GlobalFlags here and already set some config values
	// to ensure they are available. This must happen exactly once: parsing
	// again would append to the repeatable flags, doubling every `-H` and
	// `-q` value.
	if err := GlobalFlags.Parse(os.Args[1:]); err != nil {
		if err != pflag.ErrHelp {
			panic(err)
		}
	}
	if noCache, _ := GlobalFlags.GetBool("rsh-no-cache"); noCache {
		viper.Set("rsh-no-cache", true)
	}
	if verbose, _ := GlobalFlags.GetBool("rsh-verbose"); verbose {
		viper.Set("rsh-verbose", true)
	}
	if insecure, _ := GlobalFlags.GetBool("rsh-insecure"); insecure {
		viper.Set("rsh-insecure", true)
	}
	if cert, _ := GlobalFlags.GetString("rsh-client-cert"); cert != "" {
		viper.Set("rsh-client-cert", cert)
	}
	if key, _ := GlobalFlags.GetString("rsh-client-key"); key != "" {
		viper.Set("rsh-client-key", key)
	}
	if caCert, _ := GlobalFlags.GetString("rsh-ca-cert"); caCert != "" {
		viper.Set("rsh-ca-cert", caCert)
	}
	if query, _ := GlobalFlags.GetStringArray("rsh-query"); len(query) > 0 {
		viper.Set("rsh-query", query)
	}
	if headers, _ := GlobalFlags.GetStringArray("rsh-header"); len(headers) > 0 {
		viper.Set("rsh-header", headers)
	}
	profile, _ := GlobalFlags.GetString("rsh-profile")
	viper.Set("rsh-profile", profile)
	if retries, _ := GlobalFlags.GetInt("rsh-retry"); retries > 0 {
		viper.Set("rsh-retry", retries)
	}
	if timeout, _ := GlobalFlags.GetDuration("rsh-timeout"); timeout > 0 {
		viper.Set("rsh-timeout", timeout)
	}

	// Now that global flags are parsed we can enable verbose mode if requested.
	if viper.GetBool("rsh-verbose") {
		enableVerbose = true
	}

	// Auth and the project header can be redirected from the command line, so
	// they are settled here rather than in Init, but still before the API load
	// below makes the first request.
	appName := viper.GetString("app-name")
	initServiceAccount(appName)
	initProjectID(appName)
	applyCredentialOverrides()

	// `--version` is a reporting command used for support/debugging, so it
	// must work even when the API is unreachable. Report the cached spec state
	// from disk rather than triggering a network load (which would also prompt
	// for auth). Cobra prints the version for `--version`/`--version=true`; the
	// `-v` shorthand is taken by `rsh-verbose`.
	wantVersion, _ := GlobalFlags.GetBool("version")

	// Load all configured API operations directly onto the root command.
	for name, cfg := range configs {
		currentConfig = cfg
		viper.Set("api-name", name)
		currentBase := cfg.Base
		if p := cfg.Profiles[profile]; p != nil && p.Base != "" {
			currentBase = p.Base
		}
		if currentBase == "" {
			continue
		}
		if wantVersion {
			// Skip the network load; versionExtraInfo reads the on-disk cache.
			break
		}
		api, err := Load(currentBase, Root)
		if err != nil {
			panic(err)
		}
		LoadedAPI = api
		break
	}

	// Phew, we made it. Execute the command now that everything is loaded
	// and all the relevant sub-commands are registered.
	defer func() {
		if err := recover(); err != nil {
			LogError("Caught error: %v", err)
			LogDebug("%s", string(debug.Stack()))
			if e, ok := err.(error); ok {
				returnErr = e
			} else {
				returnErr = fmt.Errorf("%v", err)
			}
		}
	}()
	if err := Root.Execute(); err != nil {
		LogError("Error: %v", err)
		returnErr = err
	}

	return returnErr
}

// GetExitCode returns the exit code to use based on the last HTTP status code.
func GetExitCode() int {
	if s := GetLastStatus() / 100; s > 2 && !viper.GetBool("rsh-ignore-status-code") {
		return s
	}

	return 0
}
