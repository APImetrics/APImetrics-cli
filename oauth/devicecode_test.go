package oauth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// deviceServer stands in for Auth0's device flow: /oauth/device/code hands out
// a code, and /oauth/token replies with each of the given responses in turn.
func deviceServer(t *testing.T, tokenReplies ...string) (*httptest.Server, *url.Values, *[]url.Values) {
	t.Helper()
	deviceForm := &url.Values{}
	tokenForms := &[]url.Values{}

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/oauth/device/code":
			*deviceForm = r.PostForm
			w.Write([]byte(`{"device_code":"dev-123","user_code":"ABCD-EFGH","verification_uri":"https://auth.example.com/activate","verification_uri_complete":"https://auth.example.com/activate?user_code=ABCD-EFGH","expires_in":900,"interval":5}`))
		case "/oauth/token":
			*tokenForms = append(*tokenForms, r.PostForm)
			reply := tokenReplies[len(*tokenForms)-1]
			if reply[0] == '!' {
				w.WriteHeader(http.StatusForbidden)
				reply = reply[1:]
			}
			w.Write([]byte(reply))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(s.Close)

	return s, deviceForm, tokenForms
}

func newDeviceSource(server *httptest.Server, slept *[]time.Duration) *DeviceCodeTokenSource {
	return &DeviceCodeTokenSource{
		ClientID:       "client-1",
		DeviceURL:      server.URL + "/oauth/device/code",
		TokenURL:       server.URL + "/oauth/token",
		EndpointParams: &url.Values{"audience": []string{"https://client.apimetrics.io"}},
		Scopes:         []string{"openid profile email"},
		sleep:          func(d time.Duration) { *slept = append(*slept, d) },
	}
}

func TestDeviceCodeTokenSourcePollsUntilApproved(t *testing.T) {
	server, deviceForm, tokenForms := deviceServer(t,
		`!{"error":"authorization_pending"}`,
		`!{"error":"slow_down"}`,
		`{"access_token":"an-access-token","token_type":"Bearer","expires_in":3600,"refresh_token":"a-refresh-token"}`,
	)

	var slept []time.Duration
	token, err := newDeviceSource(server, &slept).Token()
	require.NoError(t, err)

	assert.Equal(t, "an-access-token", token.AccessToken)
	assert.Equal(t, "a-refresh-token", token.RefreshToken)
	assert.True(t, token.Valid())

	assert.Equal(t, "client-1", deviceForm.Get("client_id"))
	assert.Equal(t, "openid profile email", deviceForm.Get("scope"))
	assert.Equal(t, "https://client.apimetrics.io", deviceForm.Get("audience"))

	require.Len(t, *tokenForms, 3)
	for _, form := range *tokenForms {
		assert.Equal(t, deviceGrantType, form.Get("grant_type"))
		assert.Equal(t, "dev-123", form.Get("device_code"))
		assert.Equal(t, "client-1", form.Get("client_id"))
	}

	// The server's interval, then 5 seconds more after slow_down.
	assert.Equal(t, []time.Duration{5 * time.Second, 5 * time.Second, 10 * time.Second}, slept)
}

func TestDeviceCodeTokenSourceStopsOnTerminalErrors(t *testing.T) {
	for _, tc := range []struct{ name, reply, message string }{
		{"access denied", `!{"error":"access_denied"}`, "denied"},
		{"code expired", `!{"error":"expired_token"}`, "expired"},
		{"other oauth error", `!{"error":"invalid_grant"}`, "bad response from token endpoint"},
		{"not json", `!not json`, "bad response from token endpoint"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server, _, tokenForms := deviceServer(t, tc.reply)

			var slept []time.Duration
			_, err := newDeviceSource(server, &slept).Token()

			assert.ErrorContains(t, err, tc.message)
			assert.Len(t, *tokenForms, 1)
		})
	}
}

func TestDeviceCodeTokenSourceGivesUpWhenCodeExpires(t *testing.T) {
	pending := make([]string, 200)
	for i := range pending {
		pending[i] = `!{"error":"authorization_pending"}`
	}
	server, _, tokenForms := deviceServer(t, pending...)

	var slept []time.Duration
	_, err := newDeviceSource(server, &slept).Token()

	assert.ErrorContains(t, err, "expired")
	// 900 second lifetime at a 5 second interval.
	assert.Len(t, *tokenForms, 180)
}

func TestDeviceURLFor(t *testing.T) {
	u, err := deviceURLFor("https://auth.apimetrics.io/oauth/token")
	require.NoError(t, err)
	assert.Equal(t, "https://auth.apimetrics.io/oauth/device/code", u)
}

func TestWithAudience(t *testing.T) {
	authorize := "https://auth.apimetrics.io/authorize?audience=https://client.apimetrics.io"

	params := withAudience(url.Values{}, authorize)
	assert.Equal(t, "https://client.apimetrics.io", params.Get("audience"))

	// An explicit audience wins, and the input is left untouched.
	explicit := url.Values{"audience": []string{"https://other.example.com"}}
	assert.Equal(t, "https://other.example.com", withAudience(explicit, authorize).Get("audience"))

	empty := url.Values{}
	withAudience(empty, authorize)
	assert.Empty(t, empty)
}
