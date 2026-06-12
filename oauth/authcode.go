package oauth

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"context"

	"apicontext.com/apimetrics/cli"
	"github.com/mattn/go-isatty"
	"golang.org/x/oauth2"
)

var htmlSuccess = `
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Login Successful — APImetrics</title>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css?family=Open+Sans:300,400,600,700" rel="stylesheet">
  </head>
  <style>
    :root {
      --brand-navy: #27283c;
      --brand-blue: #00b1ff;
      --brand-green: #2ecc71;
    }
    * { box-sizing: border-box; }
    body {
      font-family: "Open Sans", -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      color: var(--brand-navy);
      background: #f9f9f9;
      margin: 0;
      min-height: 100vh;
      display: flex;
      align-items: center;
      justify-content: center;
    }
    .card {
      background: #fff;
      width: 90%;
      max-width: 420px;
      padding: 48px 40px;
      border-radius: 12px;
      text-align: center;
      box-shadow: 0 12px 40px -12px rgba(39, 40, 60, 0.25);
      animation: fade 0.6s ease-out both;
    }
    .logo { width: 64px; height: 64px; margin-bottom: 24px; }
    .icon {
      margin: 0 auto 24px;
      width: 72px;
      height: 72px;
      border-radius: 50%;
      background: var(--brand-green);
      position: relative;
      animation: pop 0.5s cubic-bezier(0.175, 0.885, 0.32, 1.275) both;
    }
    .icon .check {
      position: absolute;
      top: 19px;
      left: 25px;
      width: 18px;
      height: 30px;
      border-right: 5px solid #fff;
      border-bottom: 5px solid #fff;
      transform: rotate(45deg);
      transform-origin: center;
      animation: draw 0.4s 0.3s ease-out both;
    }
    h1 { font-size: 22px; font-weight: 700; margin: 0 0 12px; }
    p { font-size: 15px; line-height: 1.5; color: #5a5b6a; margin: 0; }
    @keyframes fade { from { opacity: 0; transform: translateY(8px); } to { opacity: 1; transform: none; } }
    @keyframes pop { from { transform: scale(0); } to { transform: scale(1); } }
    @keyframes draw { from { opacity: 0; } to { opacity: 1; } }
  </style>
  <body>
    <div class="card">
      <img class="logo" src="https://client.apimetrics.io/android-chrome-192x192.png" alt="APImetrics" onerror="this.style.display='none'">
      <div class="icon"><div class="check"></div></div>
      <h1>Login Successful!</h1>
      <p>Please return to the terminal. You may now close this window.</p>
    </div>
  </body>
</html>
`

var htmlError = `
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Login Failed — APImetrics</title>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css?family=Open+Sans:300,400,600,700" rel="stylesheet">
  </head>
  <style>
    :root {
      --brand-navy: #27283c;
      --brand-red: #e94f37;
    }
    * { box-sizing: border-box; }
    body {
      font-family: "Open Sans", -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      color: var(--brand-navy);
      background: #f9f9f9;
      margin: 0;
      min-height: 100vh;
      display: flex;
      align-items: center;
      justify-content: center;
    }
    .card {
      background: #fff;
      width: 90%;
      max-width: 420px;
      padding: 48px 40px;
      border-radius: 12px;
      text-align: center;
      box-shadow: 0 12px 40px -12px rgba(39, 40, 60, 0.25);
      animation: fade 0.6s ease-out both;
    }
    .logo { width: 64px; height: 64px; margin-bottom: 24px; }
    .icon {
      margin: 0 auto 24px;
      width: 72px;
      height: 72px;
      border-radius: 50%;
      background: var(--brand-red);
      position: relative;
      animation: pop 0.5s cubic-bezier(0.175, 0.885, 0.32, 1.275) both;
    }
    .icon .x, .icon .x:after {
      position: absolute;
      top: 33px;
      left: 22px;
      width: 28px;
      height: 5px;
      border-radius: 3px;
      background: #fff;
      transform: rotate(45deg);
    }
    .icon .x:after { content: ""; transform: rotate(90deg); top: 0; left: 0; }
    h1 { font-size: 22px; font-weight: 700; margin: 0 0 12px; }
    p { font-size: 15px; line-height: 1.5; color: #5a5b6a; margin: 0; word-break: break-word; }
    @keyframes fade { from { opacity: 0; transform: translateY(8px); } to { opacity: 1; transform: none; } }
    @keyframes pop { from { transform: scale(0); } to { transform: scale(1); } }
  </style>
  <body>
    <div class="card">
      <img class="logo" src="https://client.apimetrics.io/android-chrome-192x192.png" alt="APImetrics" onerror="this.style.display='none'">
      <div class="icon"><div class="x"></div></div>
      <h1>Login Failed</h1>
      <p><strong>$ERROR</strong><br>$DETAILS</p>
    </div>
  </body>
</html>
`

// open opens the specified URL in the default browser regardless of OS.
func open(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
		url = encodeUrlWindows(url)
	case "darwin": // mac, ios
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}

// Windows OS need strings to be encoded
// for proper command line interface handling.
func encodeUrlWindows(url string) string {
	// escape '&'
	sp := strings.Split(url, "&")
	var escaped string
	for i, p := range sp {
		// Skip adding escape at the beginning
		if i == 0 {
			escaped += p
			continue
		}
		escaped += "^&" + p
	}

	// keep protocol
	return escaped
}

// getInput waits for user input and sends it to the input channel with the
// trailing newline removed.
func getInput(input chan string) {
	r := bufio.NewReader(os.Stdin)
	result, err := r.ReadString('\n')
	if err != nil {
		panic(err)
	}

	input <- strings.TrimRight(result, "\n")
}

// authHandler is an HTTP handler that takes a channel and sends the `code`
// query param when it gets a request.
type authHandler struct {
	c chan string
}

func (h authHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	if err := r.URL.Query().Get("error"); err != "" {
		details := r.URL.Query().Get("error_description")
		rendered := strings.Replace(strings.Replace(htmlError, "$ERROR", err, 1), "$DETAILS", details, 1)
		w.Write([]byte(rendered))
		h.c <- ""
		return
	}

	h.c <- r.URL.Query().Get("code")
	w.Write([]byte(htmlSuccess))
}

// AuthorizationCodeTokenSource with PKCE as described in:
// https://www.oauth.com/oauth2-servers/pkce/
// This works by running a local HTTP server on port 8484 and then having the
// user log in through a web browser, which redirects to the redirect url with
// an authorization code. That code is then used to make another HTTP request
// to fetch an auth token (and refresh token). That token is then in turn
// used to make requests against the API.
type AuthorizationCodeTokenSource struct {
	ClientID       string
	ClientSecret   string
	AuthorizeURL   string
	TokenURL       string
	RedirectURL    string
	EndpointParams *url.Values
	Scopes         []string
}

func (ac *AuthorizationCodeTokenSource) getRedirectUrl() string {
	if ac.RedirectURL == "" {
		return "http://localhost:8484"
	}

	return ac.RedirectURL
}

// Token generates a new token using an authorization code.
func (ac *AuthorizationCodeTokenSource) Token() (*oauth2.Token, error) {
	// Generate a random code verifier string
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		return nil, err
	}

	verifier := base64.RawURLEncoding.EncodeToString(verifierBytes)

	// Generate a code challenge. Only the challenge is sent when requesting a
	// code which allows us to keep it secret for now.
	shaBytes := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(shaBytes[:])

	// Generate a URL with the challenge to have the user log in.
	authorizeURL, err := url.Parse(ac.AuthorizeURL)
	if err != nil {
		panic(err)
	}

	aq := authorizeURL.Query()
	aq.Set("response_type", "code")
	aq.Set("code_challenge", challenge)
	aq.Set("code_challenge_method", "S256")
	aq.Set("client_id", ac.ClientID)
	aq.Set("redirect_uri", ac.getRedirectUrl())
	aq.Set("scope", strings.Join(ac.Scopes, " "))
	if ac.EndpointParams != nil {
		for k, v := range *ac.EndpointParams {
			aq.Set(k, v[0])
		}
	}
	authorizeURL.RawQuery = aq.Encode()

	// Run server before opening the user's browser so we are ready for any redirect.
	codeChan := make(chan string)
	handler := authHandler{
		c: codeChan,
	}

	// strip protocol prefix from configured redirect url for local webserver
	u, err := url.Parse(ac.getRedirectUrl())
	if err != nil {
		panic(err)
	}
	redirectServer := fmt.Sprintf("%s:%s", u.Hostname(), u.Port())

	s := &http.Server{
		Addr:           redirectServer,
		Handler:        handler,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   5 * time.Second,
		MaxHeaderBytes: 1024,
	}

	go func() {
		// Run in a goroutine until the server is closed or we get an error.
		if err := s.ListenAndServe(); err != http.ErrServerClosed {
			panic(err)
		}
	}()

	// Print welcome banner, show login URL, and ask before opening browser.
	fmt.Fprint(os.Stderr, `
    _   ___ ___           _       _
   /_\ | _ \_ _|_ __  ___| |_ _ _(_)__ ___
  / _ \|  _/| || '  \/ -_|  _| '_| / _(_-<
 /_/ \_\_| |___|_|_|_\___|\__|_| |_\__/__/
`)
	fmt.Fprintln(os.Stderr, "Welcome to APImetrics CLI!")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "To log in, open the following URL in your browser:")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  "+authorizeURL.String())
	fmt.Fprintln(os.Stderr, "")
	// Only prompt interactively if stdin is a live terminal. If a file or
	// command has been piped in it is likely the request body to use after
	// auth, so we must not consume it here (see the manual-code path below).
	if isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd()) {
		fmt.Fprint(os.Stderr, "Open your browser now? [Y/n]: ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer == "" || answer == "y" || answer == "yes" {
			open(authorizeURL.String())
		}
	} else {
		open(authorizeURL.String())
	}

	// Provide a way to manually enter the code, e.g. for remote SSH sessions.
	// Only read from stdin if it is a live terminal, if a file or command has
	// been piped in it is likely the request body to use after auth.
	manualCodeChan := make(chan string)
	if isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd()) {
		fmt.Fprint(os.Stderr, "Alternatively, enter the code manually: ")
		go getInput(manualCodeChan)
	}

	// Get code from handler, exchange it for a token, and then return it. This
	// select blocks until one code becomes available.
	// There is currently no timeout.
	var code string
	select {
	case code = <-codeChan:
	case code = <-manualCodeChan:
	}
	fmt.Fprintln(os.Stderr, "")
	s.Shutdown(context.Background())

	if code == "" {
		fmt.Fprintln(os.Stderr, "Unable to get a code. See browser for details. Aborting!")
		os.Exit(1)
	}

	payload := url.Values{}
	payload.Set("grant_type", "authorization_code")
	payload.Set("client_id", ac.ClientID)
	payload.Set("code_verifier", verifier)
	payload.Set("code", code)
	payload.Set("redirect_uri", ac.getRedirectUrl())
	if ac.ClientSecret != "" {
		payload.Set("client_secret", ac.ClientSecret)
	}

	return requestToken(ac.TokenURL, payload.Encode())
}

// AuthorizationCodeHandler sets up the OAuth 2.0 authorization code with PKCE authentication
// flow.
type AuthorizationCodeHandler struct{}

// Parameters returns a list of OAuth2 Authorization Code inputs.
func (h *AuthorizationCodeHandler) Parameters() []cli.AuthParam {
	return []cli.AuthParam{
		{Name: "client_id", Required: true, Help: "OAuth 2.0 Client ID"},
		{Name: "client_secret", Required: false, Help: "OAuth 2.0 Client Secret if exists"},
		{Name: "authorize_url", Required: true, Help: "OAuth 2.0 authorization URL, e.g. https://api.example.com/oauth/authorize"},
		{Name: "token_url", Required: true, Help: "OAuth 2.0 token URL, e.g. https://api.example.com/oauth/token"},
		{Name: "scopes", Help: "Optional scopes to request in the token"},
		{Name: "redirect_url", Help: "Optional redirect URL with protocol and port, defaults to 'http://localhost:8484' if not specified. "},
	}
}

// OnRequest gets run before the request goes out on the wire.
func (h *AuthorizationCodeHandler) OnRequest(request *http.Request, key string, params map[string]string) error {
	if request.Header.Get("Authorization") == "" {
		endpointParams := url.Values{}
		for k, v := range params {
			if k == "client_id" || k == "client_secret" || k == "scopes" || k == "authorize_url" || k == "token_url" || k == "redirect_url" {
				// Not a custom param...
				continue
			}

			endpointParams.Add(k, v)
		}

		source := &AuthorizationCodeTokenSource{
			ClientID:       params["client_id"],
			ClientSecret:   params["client_secret"],
			AuthorizeURL:   params["authorize_url"],
			TokenURL:       params["token_url"],
			RedirectURL:    params["redirect_url"],
			EndpointParams: &endpointParams,
			Scopes:         strings.Split(params["scopes"], ","),
		}

		// Try to get a cached refresh token from the current profile and use
		// it to wrap the auth code token source with a refreshing source.
		refreshKey := key + ".refresh"
		refreshSource := RefreshTokenSource{
			ClientID:       params["client_id"],
			TokenURL:       params["token_url"],
			Scopes:         strings.Split(params["scopes"], ","),
			EndpointParams: &endpointParams,
			RefreshToken:   cli.Cache.GetString(refreshKey),
			TokenSource:    source,
		}

		return TokenHandler(&refreshSource, key, request)
	}

	return nil
}
