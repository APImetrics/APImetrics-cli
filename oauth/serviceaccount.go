package oauth

import (
	"net/http"
	"net/url"

	"apicontext.com/apimetrics/cli"
	"golang.org/x/oauth2"
)

// ServiceAccountTokenSource fetches an access token with the OAuth 2.0 client
// credentials grant, using the client ID and secret from an APImetrics service
// account file.
//
// This grant never issues a refresh token (RFC 6749 §4.4.3) — the client
// credentials are themselves long-lived, so an expired access token is
// replaced by simply asking for a new one.
type ServiceAccountTokenSource struct {
	// ClientID of the service account.
	ClientID string

	// ClientSecret of the service account.
	ClientSecret string

	// TokenURL of the APImetrics auth service.
	TokenURL string

	// Audience the token is requested for.
	Audience string
}

// Token requests a new access token from the auth service.
func (ts *ServiceAccountTokenSource) Token() (*oauth2.Token, error) {
	payload := url.Values{}
	payload.Set("grant_type", "client_credentials")
	payload.Set("client_id", ts.ClientID)
	payload.Set("client_secret", ts.ClientSecret)
	if ts.Audience != "" {
		payload.Set("audience", ts.Audience)
	}

	return requestToken(ts.TokenURL, payload.Encode())
}

// ServiceAccountHandler authenticates requests with an APImetrics service
// account instead of an interactive browser login.
type ServiceAccountHandler struct{}

// Parameters returns a list of service account auth inputs.
func (h *ServiceAccountHandler) Parameters() []cli.AuthParam {
	return []cli.AuthParam{
		{Name: "client_id", Required: true, Help: "Service account client ID"},
		{Name: "client_secret", Required: true, Help: "Service account client secret"},
		{Name: "token_url", Required: true, Help: "OAuth 2.0 token URL, e.g. https://auth.apimetrics.io/oauth/token"},
		{Name: "audience", Help: "OAuth 2.0 audience to request the token for"},
		{Name: "cache_key", Help: "Key the access token is cached under, derived from the credentials"},
	}
}

// OnRequest gets run before the request goes out on the wire.
func (h *ServiceAccountHandler) OnRequest(request *http.Request, key string, params map[string]string) error {
	if request.Header.Get("Authorization") != "" {
		return nil
	}

	if params["client_id"] == "" || params["client_secret"] == "" || params["token_url"] == "" {
		return ErrInvalidProfile
	}

	source := &ServiceAccountTokenSource{
		ClientID:     params["client_id"],
		ClientSecret: params["client_secret"],
		TokenURL:     params["token_url"],
		Audience:     params["audience"],
	}

	// Cache the token against the credentials rather than the profile, so that
	// tokens for different service accounts — and for an interactive login —
	// never overwrite each other.
	if cacheKey := params["cache_key"]; cacheKey != "" {
		key = cacheKey
	}

	return TokenHandler(source, key, request)
}
