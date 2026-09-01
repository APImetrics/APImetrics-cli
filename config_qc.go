//go:build !prod && !dev && !qcstable && !beta

package main

var envName = "qc"
var appName = "apimetrics-qc"
var baseURL = "https://qc-client.apimetrics.io"
var authURL = "https://qc-auth.apimetrics.io/authorize"
var tokenURL = "https://qc-auth.apimetrics.io/oauth/token"
var authAudience = "https://client.apimetrics.io"
var clientID = "4fhqu4lEH5ExaRh00X1B9WJSkjTnUmuK"
