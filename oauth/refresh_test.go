package oauth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// fallbackSource stands in for the browser login.
type fallbackSource struct {
	token *oauth2.Token
	calls int
}

func (s *fallbackSource) Token() (*oauth2.Token, error) {
	s.calls++
	return s.token, nil
}

func TestRefreshTokenSourceUsesRefreshToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "refresh_token", r.PostForm.Get("grant_type"))
		assert.Equal(t, "client-1", r.PostForm.Get("client_id"))
		assert.Equal(t, "old-refresh", r.PostForm.Get("refresh_token"))
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"new-access","token_type":"Bearer","expires_in":3600,"refresh_token":"rotated-refresh"}`))
	}))
	defer srv.Close()

	fallback := &fallbackSource{}
	ts := &RefreshTokenSource{
		ClientID:     "client-1",
		TokenURL:     srv.URL,
		RefreshToken: "old-refresh",
		TokenSource:  fallback,
	}

	token, err := ts.Token()
	require.NoError(t, err)
	assert.Equal(t, "new-access", token.AccessToken)
	assert.Equal(t, "rotated-refresh", token.RefreshToken)
	assert.Zero(t, fallback.calls, "a working refresh token must not trigger a browser login")
}

func TestRefreshTokenSourceFallsBackWhenRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer srv.Close()

	fallback := &fallbackSource{token: &oauth2.Token{AccessToken: "login-access", RefreshToken: "login-refresh"}}
	ts := &RefreshTokenSource{
		ClientID:     "client-1",
		TokenURL:     srv.URL,
		RefreshToken: "expired-refresh",
		TokenSource:  fallback,
	}

	token, err := ts.Token()
	require.NoError(t, err)
	assert.Equal(t, "login-access", token.AccessToken)
	assert.Equal(t, 1, fallback.calls)
	assert.Equal(t, "login-refresh", ts.RefreshToken)
}
