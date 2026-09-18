//go:build beta && !prod && !dev && !qcstable

package main

// Beta runs against production Auth0 (same tenant, audience and client ID as
// config_prod.go) but points at the beta API host, so a beta login is a real
// production login. It still gets its own config directory, so the token is
// stored separately from the production build's.
var envName = "beta"
var appName = "apimetrics-beta"
var baseURL = "https://beta-client.apimetrics.io"
var authURL = "https://auth.apimetrics.io/authorize"
var tokenURL = "https://auth.apimetrics.io/oauth/token"
var authAudience = "https://client.apimetrics.io"
var clientID = "dPbV4VPvioF4nZ3oGQMn7n1vE2pFNAAI"
