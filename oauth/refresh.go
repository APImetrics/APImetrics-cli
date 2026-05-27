package oauth

import (
	"net/url"
	"strings"

	"apicontext.com/apimetrics/cli"
	"golang.org/x/oauth2"
)

// RefreshTokenSource will use a refresh token to try and get a new token before
// calling the original token source to get a new token.
type RefreshTokenSource struct {
	// ClientID of the application
	ClientID string

	// TokenURL is used to fetch new tokens
	TokenURL string

	// Scopes to request when refreshing the token
	Scopes []string

	// EndpointParams are extra URL query parameters to include in the request
	EndpointParams *url.Values

	// RefreshToken from a cache, if available. If not, then the first time a
	// token is requested it will be loaded from the token source and this value
	// will get updated if it's present in the returned token.
	RefreshToken string

	// TokenSource to wrap to fetch new tokens if the refresh token is missing or
	// did not work to get a new token.
	TokenSource oauth2.TokenSource
}

// Token generates a new token using either a refresh token or by falling
// back to the original source.
func (ts *RefreshTokenSource) Token() (*oauth2.Token, error) {
	if ts.RefreshToken != "" {
		cli.LogDebug("Trying refresh token to get a new access token")
		refreshParams := url.Values{
			"grant_type":    []string{"refresh_token"},
			"client_id":     []string{ts.ClientID},
			"refresh_token": []string{ts.RefreshToken},
			"scope":         []string{strings.Join(ts.Scopes, " ")},
		}

		// Copy any endpoint-specific parameters.
		if ts.EndpointParams != nil {
			for k, v := range *ts.EndpointParams {
				refreshParams[k] = v
			}
		}

		token, err := requestToken(ts.TokenURL, refreshParams.Encode())
		if err == nil {
			return token, err
		}

		// Couldn't refresh, try the original source again.
	}

	token, err := ts.TokenSource.Token()
	if err != nil {
		return nil, err
	}

	// Update the initial token with the (possibly new) refresh token.
	ts.RefreshToken = token.RefreshToken

	return token, nil
}
