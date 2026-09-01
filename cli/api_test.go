package cli

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

type overrideLoader struct {
	detect        func(resp *http.Response) bool
	load          func(entrypoint, spec url.URL, resp *http.Response) (API, error)
	locationHints func() []string
}

func (l *overrideLoader) Detect(resp *http.Response) bool {
	if l.detect != nil {
		return l.detect(resp)
	}
	return true
}

func (l *overrideLoader) Load(entrypoint url.URL, spec url.URL, resp *http.Response) (API, error) {
	if l.load != nil {
		return l.load(entrypoint, spec, resp)
	}
	return API{}, nil
}
func (l *overrideLoader) LocationHints() []string {
	if l.locationHints != nil {
		return l.locationHints()
	}
	return []string{}
}

func TestLoadFromFile(t *testing.T) {
	reset(false)
	viper.Set("rsh-no-cache", true)
	AddLoader(&overrideLoader{
		load: func(entrypoint, spec url.URL, resp *http.Response) (API, error) {
			assert.Equal(t, "testdata/petstore.json", spec.String())
			return API{}, nil
		},
	})

	configs["file-load-test"] = &APIConfig{
		Base:      "https://api.example.com",
		SpecFiles: []string{"testdata/petstore.json"},
	}

	_, err := Load("https://api.example.com", &cobra.Command{})

	assert.NoError(t, err)
}

func TestBadSpecURL(t *testing.T) {
	reset(false)
	viper.Set("rsh-no-cache", true)
	AddLoader(&overrideLoader{
		load: func(entrypoint, spec url.URL, resp *http.Response) (API, error) {
			assert.Equal(t, "testdata/petstore.json", spec.String())
			return API{}, nil
		},
	})

	configs["bad-spec-url-test"] = &APIConfig{
		Base:      "https://api.example.com",
		SpecFiles: []string{"http://abc{def@ghi}"},
	}

	_, err := Load("https://api.example.com", &cobra.Command{})
	assert.Error(t, err)
}

func TestVersionExtraInfo(t *testing.T) {
	// Isolate the cache dir (getCacheDir uses <APPNAME>_CACHE_DIR; app-name is
	// "test" via reset -> Init).
	os.Setenv("TEST_CACHE_DIR", t.TempDir())
	defer os.Unsetenv("TEST_CACHE_DIR")

	reset(false)
	viper.Set("api-name", "vtest")

	writeCache := func(cliVersion, specVersion string) {
		b, err := cbor.Marshal(&API{CLIVersion: cliVersion, SpecVersion: specVersion})
		assert.NoError(t, err)
		assert.NoError(t, os.WriteFile(filepath.Join(getCacheDir(), "vtest.cbor"), b, 0o600))
		Cache.Set("vtest.checked", time.Now())
		assert.NoError(t, Cache.WriteConfig())
	}

	// No cache on disk yet -> unknown / never.
	out := versionExtraInfo()
	assert.Contains(t, out, "API spec version: unknown")
	assert.Contains(t, out, "Spec last updated:  never")

	// Cache written by the current CLI version -> reported.
	writeCache(Root.Version, "9.9.9")
	out = versionExtraInfo()
	assert.Contains(t, out, "API spec version: 9.9.9")
	assert.NotContains(t, out, "Spec last updated:  never")

	// Cache written by a different CLI version (e.g. after a binary upgrade) is
	// stale and must not be reported.
	writeCache("some-older-version", "1.2.3")
	out = versionExtraInfo()
	assert.Contains(t, out, "API spec version: unknown")
	assert.Contains(t, out, "Spec last updated:  never")
}
