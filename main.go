package main

import (
	"os"

	"apicontext.com/apimetrics/bulk"
	"apicontext.com/apimetrics/cli"
	"apicontext.com/apimetrics/oauth"
	"apicontext.com/apimetrics/openapi"
	"apicontext.com/apimetrics/skills"
)

var version string = "dev"

// envName, appName, baseURL, authURL, tokenURL, authAudience and clientID are
// defined in exactly one config_*.go, selected by build tag:
//
//	(none)     config_qc.go        qc          apimetrics-qc
//	prod       config_prod.go      production  apimetrics
//	beta       config_beta.go      beta        apimetrics-beta
//	qcstable   config_qc_stable.go qc-stable   apimetrics-qc-stable
//	dev        config_dev.go       dev         apimetrics-dev
//
// appName is both the installed command name and the name of the config and
// cache directories, so builds for different environments can be installed
// side by side and hold independent logins.

func main() {
	if version == "dev" {
		// Try to add the executable modification time to the dev version.
		filename, _ := os.Executable()
		if info, err := os.Stat(filename); err == nil {
			version += "-" + info.ModTime().Format("2006-01-02-15:04")
		}
	}

	cli.SetBuildConfig(baseURL, authURL, tokenURL, authAudience, clientID)
	cli.SetEnvironment(envName)
	cli.Init(appName, version)

	// Register default encodings, content type handlers, and link parsers.
	cli.Defaults()

	bulk.Init(cli.Root)
	skills.Init(cli.Root)

	// Register format loaders to auto-discover API descriptions
	cli.AddLoader(openapi.New())

	// Register auth schemes
	cli.AddAuth("oauth-client-credentials", &oauth.ClientCredentialsHandler{})
	cli.AddAuth("oauth-authorization-code", &oauth.AuthorizationCodeHandler{})
	cli.AddAuth("oauth-service-account", &oauth.ServiceAccountHandler{})

	// Run the CLI, parsing arguments, making requests, and printing responses.
	if err := cli.Run(); err != nil {
		os.Exit(1)
	}

	// Exit based on the status code of the last request.
	os.Exit(cli.GetExitCode())
}
