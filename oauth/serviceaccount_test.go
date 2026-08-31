package oauth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"apicontext.com/apimetrics/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tokenServer stands in for the APImetrics auth service, recording the form it
// was posted and handing back a token.
func tokenServer(t *testing.T, accessToken string) (*httptest.Server, *url.Values) {
	t.Helper()
	posted := &url.Values{}

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		*posted = r.PostForm

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token": "` + accessToken + `", "token_type": "Bearer", "expires_in": 3600}`))
	}))
	t.Cleanup(s.Close)

	return s, posted
}

func TestServiceAccountTokenSource(t *testing.T) {
	server, posted := tokenServer(t, "an-access-token")

	source := &ServiceAccountTokenSource{
		ClientID:     "abc123",
		ClientSecret: "s3cret",
		TokenURL:     server.URL,
		Audience:     "https://client.apimetrics.io",
	}

	token, err := source.Token()
	require.NoError(t, err)

	assert.Equal(t, "an-access-token", token.AccessToken)
	assert.Equal(t, "Bearer", token.TokenType)
	assert.True(t, token.Valid())

	assert.Equal(t, "client_credentials", posted.Get("grant_type"))
	assert.Equal(t, "abc123", posted.Get("client_id"))
	assert.Equal(t, "s3cret", posted.Get("client_secret"))
	assert.Equal(t, "https://client.apimetrics.io", posted.Get("audience"))

	// The client credentials grant issues no refresh token (RFC 6749 §4.4.3),
	// and asking for offline_access is rejected, so nothing requests a scope.
	assert.Empty(t, token.RefreshToken)
	assert.NotContains(t, *posted, "scope")
}

func TestServiceAccountTokenSourceOmitsEmptyAudience(t *testing.T) {
	server, posted := tokenServer(t, "an-access-token")

	source := &ServiceAccountTokenSource{ClientID: "abc123", ClientSecret: "s3cret", TokenURL: server.URL}
	_, err := source.Token()
	require.NoError(t, err)

	assert.NotContains(t, *posted, "audience")
}

func TestServiceAccountTokenSourceBadResponse(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "access_denied"}`))
	}))
	defer s.Close()

	source := &ServiceAccountTokenSource{ClientID: "abc123", ClientSecret: "wrong", TokenURL: s.URL}
	_, err := source.Token()

	assert.ErrorContains(t, err, "bad response from token endpoint")
	assert.ErrorContains(t, err, "access_denied")
}

func TestServiceAccountHandlerOnRequest(t *testing.T) {
	t.Setenv("APIMETRICS_TEST_CONFIG_DIR", t.TempDir())
	t.Setenv("APIMETRICS_TEST_CACHE_DIR", t.TempDir())
	cli.Init("apimetrics-test", "1.0.0")

	server, _ := tokenServer(t, "an-access-token")

	handler := &ServiceAccountHandler{}
	params := map[string]string{
		"client_id":     "abc123",
		"client_secret": "s3cret",
		"token_url":     server.URL,
		"audience":      "https://client.apimetrics.io",
		"cache_key":     "service-account:0123456789abcdef",
	}

	req, _ := http.NewRequest(http.MethodGet, "https://api.example.com/", nil)
	require.NoError(t, handler.OnRequest(req, "apimetrics:default", params))

	assert.Equal(t, "Bearer an-access-token", req.Header.Get("Authorization"))

	// The token is cached under the credential-derived key, leaving the
	// interactive login's key untouched.
	assert.Equal(t, "an-access-token", cli.Cache.GetString("service-account:0123456789abcdef.token"))
	assert.Empty(t, cli.Cache.GetString("apimetrics:default.token"))
}

func TestServiceAccountHandlerSkipsExistingAuth(t *testing.T) {
	handler := &ServiceAccountHandler{}

	req, _ := http.NewRequest(http.MethodGet, "https://api.example.com/", nil)
	req.Header.Set("Authorization", "Bearer already-set")

	// No token URL, so this would fail if the handler tried to fetch a token.
	require.NoError(t, handler.OnRequest(req, "apimetrics:default", map[string]string{}))
	assert.Equal(t, "Bearer already-set", req.Header.Get("Authorization"))
}

func TestServiceAccountHandlerRequiresCredentials(t *testing.T) {
	handler := &ServiceAccountHandler{}

	for name, params := range map[string]map[string]string{
		"no client id":     {"client_secret": "s3cret", "token_url": "https://auth.example.com"},
		"no client secret": {"client_id": "abc123", "token_url": "https://auth.example.com"},
		"no token url":     {"client_id": "abc123", "client_secret": "s3cret"},
	} {
		t.Run(name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, "https://api.example.com/", nil)
			assert.ErrorIs(t, handler.OnRequest(req, "apimetrics:default", params), ErrInvalidProfile)
		})
	}
}
