package oauth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"apicontext.com/apimetrics/cli"
	"golang.org/x/oauth2"
)

// deviceGrantType is the token request grant type for the device flow.
const deviceGrantType = "urn:ietf:params:oauth:grant-type:device_code"

// deviceAuthResponse is the device authorization endpoint's reply
// (RFC 8628 §3.2).
type deviceAuthResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// DeviceCodeTokenSource implements the OAuth 2.0 device authorization grant
// (RFC 8628). The user confirms a short code in a browser on any device while
// the CLI polls the token endpoint, so no local redirect server is needed.
// This suits remote shells, where the browser can't reach localhost.
type DeviceCodeTokenSource struct {
	ClientID string

	// DeviceURL is the device authorization endpoint.
	DeviceURL string

	// TokenURL is polled until the user has approved the request.
	TokenURL string

	// EndpointParams are extra parameters, such as the audience, to send to
	// the device authorization endpoint.
	EndpointParams *url.Values

	Scopes []string

	// sleep waits between polls; tests replace it to run without delay.
	sleep func(time.Duration)
}

// Token starts a device authorization and waits for the user to approve it.
func (ds *DeviceCodeTokenSource) Token() (*oauth2.Token, error) {
	params := url.Values{}
	if ds.EndpointParams != nil {
		for k, v := range *ds.EndpointParams {
			params[k] = v
		}
	}
	params.Set("client_id", ds.ClientID)
	params.Set("scope", strings.Join(ds.Scopes, " "))

	auth, err := requestDeviceCode(ds.DeviceURL, params.Encode())
	if err != nil {
		return nil, err
	}

	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "To log in, open the following URL in a browser on any device:")
	fmt.Fprintln(os.Stderr, "")
	if auth.VerificationURIComplete != "" {
		fmt.Fprintln(os.Stderr, "  "+auth.VerificationURIComplete)
	} else {
		fmt.Fprintln(os.Stderr, "  "+auth.VerificationURI)
	}
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "and confirm that it shows this code: "+auth.UserCode)
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Waiting for approval...")

	return ds.poll(auth)
}

// poll requests a token at the server's interval until the user approves or
// denies the request, or the device code expires (RFC 8628 §3.4, §3.5).
func (ds *DeviceCodeTokenSource) poll(auth *deviceAuthResponse) (*oauth2.Token, error) {
	sleep := ds.sleep
	if sleep == nil {
		sleep = time.Sleep
	}

	interval := time.Duration(auth.Interval) * time.Second
	if interval <= 0 {
		// RFC 8628 §3.2: clients must use 5 seconds if no interval is given.
		interval = 5 * time.Second
	}

	payload := url.Values{
		"grant_type":  []string{deviceGrantType},
		"device_code": []string{auth.DeviceCode},
		"client_id":   []string{ds.ClientID},
	}.Encode()

	// Stop after the code's lifetime rather than relying on the server to say
	// expired_token. The wait is summed from the intervals rather than read
	// from the clock, so tests that skip the sleep still terminate.
	lifetime := time.Duration(auth.ExpiresIn) * time.Second
	if lifetime <= 0 {
		lifetime = 15 * time.Minute
	}
	for waited := time.Duration(0); waited < lifetime; waited += interval {
		sleep(interval)

		token, code, err := requestDeviceToken(ds.TokenURL, payload)
		switch code {
		case "":
			if err != nil {
				return nil, err
			}
			return token, nil
		case "authorization_pending":
			continue
		case "slow_down":
			// RFC 8628 §3.5: increase the interval by 5 seconds.
			interval += 5 * time.Second
			continue
		case "access_denied":
			return nil, errors.New("login was denied in the browser")
		case "expired_token":
			return nil, errors.New("the login code expired before it was approved, please try again")
		default:
			return nil, err
		}
	}

	return nil, errors.New("the login code expired before it was approved, please try again")
}

// requestDeviceCode asks the device authorization endpoint for a device code
// and the user code to show.
func requestDeviceCode(deviceURL, payload string) (*deviceAuthResponse, error) {
	body, status, err := postForm(deviceURL, payload)
	if err != nil {
		return nil, err
	}

	if status != http.StatusOK {
		return nil, fmt.Errorf("bad response from device authorization endpoint:\n%s", body)
	}

	auth := &deviceAuthResponse{}
	if err := json.Unmarshal(body, auth); err != nil {
		return nil, err
	}

	if auth.DeviceCode == "" || auth.UserCode == "" {
		return nil, fmt.Errorf("device authorization endpoint returned no code:\n%s", body)
	}

	return auth, nil
}

// requestDeviceToken polls the token endpoint once. On an OAuth error it
// returns the error code, e.g. authorization_pending, so the caller can decide
// whether to keep polling.
func requestDeviceToken(tokenURL, payload string) (*oauth2.Token, string, error) {
	body, status, err := postForm(tokenURL, payload)
	if err != nil {
		return nil, "", err
	}

	if status != http.StatusOK {
		var oauthErr struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(body, &oauthErr) == nil && oauthErr.Error != "" {
			return nil, oauthErr.Error, fmt.Errorf("bad response from token endpoint:\n%s", body)
		}
		return nil, "", fmt.Errorf("bad response from token endpoint:\n%s", body)
	}

	token, err := parseToken(body)
	return token, "", err
}

// deviceURLFor derives the device authorization endpoint from the token URL,
// following Auth0's layout: https://<tenant>/oauth/token becomes
// https://<tenant>/oauth/device/code.
func deviceURLFor(tokenURL string) (string, error) {
	u, err := url.Parse(tokenURL)
	if err != nil {
		return "", err
	}
	u.Path = "/oauth/device/code"
	u.RawQuery = ""
	return u.String(), nil
}

// withAudience returns a copy of params with the audience added from the
// authorize URL's query string, which is where the browser login carries it,
// unless params already has one.
func withAudience(params url.Values, authorizeURL string) url.Values {
	out := url.Values{}
	for k, v := range params {
		out[k] = v
	}

	if out.Get("audience") == "" && authorizeURL != "" {
		if u, err := url.Parse(authorizeURL); err == nil {
			if audience := u.Query().Get("audience"); audience != "" {
				out.Set("audience", audience)
			}
		}
	}

	return out
}

// DeviceCodeHandler sets up the OAuth 2.0 device authorization grant. It takes
// the same parameters as the authorization code handler so the CLI can switch
// between them for one login and share the cached token.
type DeviceCodeHandler struct{}

// Parameters returns a list of OAuth2 device authorization inputs.
func (h *DeviceCodeHandler) Parameters() []cli.AuthParam {
	return []cli.AuthParam{
		{Name: "client_id", Required: true, Help: "OAuth 2.0 Client ID"},
		{Name: "token_url", Required: true, Help: "OAuth 2.0 token URL, e.g. https://api.example.com/oauth/token"},
		{Name: "device_url", Help: "Optional device authorization URL, defaults to /oauth/device/code on the token URL's host"},
		{Name: "authorize_url", Help: "Optional authorization URL; an audience in its query string is sent with the device request"},
		{Name: "scopes", Help: "Optional scopes to request in the token"},
	}
}

// OnRequest gets run before the request goes out on the wire.
func (h *DeviceCodeHandler) OnRequest(request *http.Request, key string, params map[string]string) error {
	if request.Header.Get("Authorization") != "" {
		return nil
	}

	deviceURL := params["device_url"]
	if deviceURL == "" {
		var err error
		if deviceURL, err = deviceURLFor(params["token_url"]); err != nil {
			return err
		}
	}

	endpointParams := url.Values{}
	for k, v := range params {
		switch k {
		case "client_id", "client_secret", "scopes", "authorize_url", "token_url", "redirect_url", "device_url":
			// Not a custom param...
			continue
		}
		endpointParams.Add(k, v)
	}

	// Only the device request gets the audience: the refresh request below
	// matches the browser login's, which never sends one.
	deviceParams := withAudience(endpointParams, params["authorize_url"])

	source := &DeviceCodeTokenSource{
		ClientID:       params["client_id"],
		DeviceURL:      deviceURL,
		TokenURL:       params["token_url"],
		EndpointParams: &deviceParams,
		Scopes:         strings.Split(params["scopes"], ","),
	}

	// As with the browser login, try a cached refresh token first.
	refreshSource := RefreshTokenSource{
		ClientID:       params["client_id"],
		TokenURL:       params["token_url"],
		Scopes:         strings.Split(params["scopes"], ","),
		EndpointParams: &endpointParams,
		RefreshToken:   cli.Cache.GetString(key + ".refresh"),
		TokenSource:    source,
	}

	return TokenHandler(&refreshSource, key, request)
}
