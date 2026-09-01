package cli

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/h2non/gock.v1"
)

func reset(color bool) {
	viper.Reset()
	viper.Set("tty", true)
	if color {
		viper.Set("color", true)
	} else {
		viper.Set("nocolor", true)
	}

	// Most tests are easier to write without retries.
	viper.Set("rsh-retry", 0)

	Init("test", "1.0.0'")
	Defaults()
}

func runNoReset(cmd string) string {
	capture := &strings.Builder{}
	Stdout = capture
	Stderr = capture
	Root.SetOut(capture)
	os.Args = strings.Split("restish "+cmd, " ")
	Run()

	return capture.String()
}

func TestLoadCache(t *testing.T) {
	// Invalidate any existin cache.
	Cache.Set("cache-test.expires", time.Now().Add(-24*time.Hour))
	Cache.WriteConfig()
	defer gock.Off()

	// Only *one* set of remote requests should be made. After that it should be
	// using the cache.
	gock.New("https://example.com/").Reply(404)
	gock.New("https://example.com/openapi.json").Reply(200).JSON(map[string]any{
		"openapi": "3.0.0",
	})

	reset(false)
	configs["cache-test"] = &APIConfig{
		name: "cache-test",
		Base: "https://example.com",
		Profiles: map[string]*APIProfile{
			"default": {},
		},
	}
	cmd := &cobra.Command{
		Use: "cache-test",
	}
	Root.AddCommand(cmd)

	AddLoader(&testLoader{
		API: API{
			Short:      "Cache Test API",
			Operations: []Operation{},
		},
	})

	// First run will generate the cache.
	runNoReset("cache-test --help")

	// These runs should *not* make any remote requests. If they do, then
	// gock will panic as only one call is mocked above.
	runNoReset("cache-test --help")
	runNoReset("cache-test --help")
}
