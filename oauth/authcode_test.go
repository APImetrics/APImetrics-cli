package oauth

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodeUrlWindowsSuccess(t *testing.T) {
	u := "https://mydomain.auth.us-east-1.amazoncognito.com/oauth2/authorize?response_type=code&client_id=1example23456789&redirect_uri=https://www.example.com&state=abcdefg&scope=openid+profile"

	r := encodeUrlWindows(u)
	//t.Log(r)

	assert.NotEqual(t, u, r)
	assert.Contains(t, r, "^&")
	assert.False(t, strings.HasPrefix(r, "^&"))
	assert.False(t, strings.HasSuffix(r, "^&"))
}

// serveAuthCallback runs the local redirect handler against the given callback
// query string and returns the rendered HTML.
func serveAuthCallback(t *testing.T, query string) string {
	t.Helper()

	// The handler blocks until the code channel is drained, so buffer it.
	h := authHandler{c: make(chan string, 1)}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/?"+query, nil))

	body, err := io.ReadAll(w.Result().Body)
	assert.NoError(t, err)

	return string(body)
}

func TestAuthHandlerEscapesErrorParams(t *testing.T) {
	body := serveAuthCallback(t,
		"error=%3Cscript%3Ealert(1)%3C%2Fscript%3E&error_description=%3Cimg+src%3Dx+onerror%3Dalert(2)%3E")

	// The attacker-controlled markup must be escaped, not reflected verbatim.
	assert.NotContains(t, body, "<script>alert(1)</script>")
	assert.NotContains(t, body, "<img src=x onerror=alert(2)>")
	assert.Contains(t, body, "&lt;script&gt;alert(1)&lt;/script&gt;")
	assert.Contains(t, body, "&lt;img src=x onerror=alert(2)&gt;")

	// Placeholders are still substituted.
	assert.NotContains(t, body, "$ERROR")
	assert.NotContains(t, body, "$DETAILS")
}

func TestAuthHandlerEscapesQuotesAndAmpersands(t *testing.T) {
	body := serveAuthCallback(t, "error=%22onload%3D%27x%27&error_description=a%26b")

	assert.Contains(t, body, "&#34;onload=&#39;x&#39;")
	assert.Contains(t, body, "a&amp;b")
}

func TestAuthHandlerPassesCodeThrough(t *testing.T) {
	h := authHandler{c: make(chan string, 1)}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/?code=abc123", nil))

	assert.Equal(t, "abc123", <-h.c)

	body, err := io.ReadAll(w.Result().Body)
	assert.NoError(t, err)
	assert.Equal(t, htmlSuccess, string(body))
}
