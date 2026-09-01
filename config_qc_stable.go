//go:build qcstable && !prod && !dev

package main

var envName = "qc-stable"
var appName = "apimetrics-qc-stable"
var baseURL = "https://qc-stable.apimetrics.io"
var authURL = "https://qc-auth.apimetrics.io/authorize"
var tokenURL = "https://qc-auth.apimetrics.io/oauth/token"
var authAudience = "https://client.apimetrics.io"
var clientID = "4fhqu4lEH5ExaRh00X1B9WJSkjTnUmuK"
