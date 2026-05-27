package main

import (
	"os"

	"apicontext.com/apimetrics/bulk"
	"apicontext.com/apimetrics/cli"
	"apicontext.com/apimetrics/oauth"
	"apicontext.com/apimetrics/openapi"
)

var version string = "dev"
var commit string
var date string

// Build-time API configuration — override with -ldflags at build time:
//
//	go build -ldflags "-X main.baseURL=https://client.apimetrics.io ..."
var baseURL = "https://qc-client.apimetrics.io"
var authURL = "https://qc-auth.apimetrics.io/authorize"
var tokenURL = "https://qc-auth.apimetrics.io/oauth/token"
var authAudience = "https://client.apimetrics.io"
var clientID = "bj0yh0AjBMzfeOpffmCj5UP8FbmYDwcM"

func main() {
	if version == "dev" {
		// Try to add the executable modification time to the dev version.
		filename, _ := os.Executable()
		if info, err := os.Stat(filename); err == nil {
			version += "-" + info.ModTime().Format("2006-01-02-15:04")
		}
	}

	cli.SetBuildConfig(baseURL, authURL, tokenURL, authAudience, clientID)
	cli.Init("apimetrics", version)

	// Register default encodings, content type handlers, and link parsers.
	cli.Defaults()

	bulk.Init(cli.Root)

	// Register format loaders to auto-discover API descriptions
	cli.AddLoader(openapi.New())

	// Register auth schemes
	cli.AddAuth("oauth-client-credentials", &oauth.ClientCredentialsHandler{})
	cli.AddAuth("oauth-authorization-code", &oauth.AuthorizationCodeHandler{})

	// Run the CLI, parsing arguments, making requests, and printing responses.
	if err := cli.Run(); err != nil {
		os.Exit(1)
	}

	// Exit based on the status code of the last request.
	os.Exit(cli.GetExitCode())
}
