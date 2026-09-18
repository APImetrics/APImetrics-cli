//go:build dev && !prod

package main

var envName = "dev"
var appName = "apimetrics-dev"
var baseURL = "http://localhost:8080"
var authURL = "https://local-apimetrics.auth0.com/authorize"
var tokenURL = "https://local-apimetrics.auth0.com/oauth/token"
var authAudience = "https://apimetrics-qc.appspot.com"
var clientID = "bpplXMDCn187JipRLl6Y9KrsTVZCJTbS"
