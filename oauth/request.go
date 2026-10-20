package oauth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"apicontext.com/apimetrics/cli"
	"golang.org/x/oauth2"
)

// tokenResponse is used to parse responses from token providers and make sure
// the expiration time is set properly regardless of whether `expires_in` or
// `expiry` is returned.
type tokenResponse struct {
	TokenType    string        `json:"token_type"`
	AccessToken  string        `json:"access_token"`
	RefreshToken string        `json:"refresh_token,omitempty"`
	ExpiresIn    time.Duration `json:"expires_in"`
	Expiry       time.Time     `json:"expiry,omitempty"`
}

// requestToken from the given URL with the given payload. This can be used
// for many different grant types and will return a parsed token.
func requestToken(tokenURL, payload string) (*oauth2.Token, error) {
	body, status, err := postForm(tokenURL, payload)
	if err != nil {
		return nil, err
	}

	if status > 200 {
		return nil, fmt.Errorf("bad response from token endpoint:\n%s", body)
	}

	return parseToken(body)
}

// parseToken decodes a successful token endpoint response.
func parseToken(body []byte) (*oauth2.Token, error) {
	decoded := tokenResponse{}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, err
	}

	expiry := decoded.Expiry
	if expiry.IsZero() {
		expiry = time.Now().Add(decoded.ExpiresIn * time.Second)
	}

	token := &oauth2.Token{
		AccessToken:  decoded.AccessToken,
		TokenType:    decoded.TokenType,
		RefreshToken: decoded.RefreshToken,
		Expiry:       expiry,
	}

	return token, nil
}

// postForm sends a form-encoded POST and returns the body and status code.
func postForm(target, payload string) ([]byte, int, error) {
	req, err := http.NewRequest(http.MethodPost, target, strings.NewReader(payload))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Add("content-type", "application/x-www-form-urlencoded")

	cli.LogDebugRequest(req)

	start := time.Now()
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	cli.LogDebugResponse(start, res)
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, 0, err
	}

	return body, res.StatusCode, nil
}
