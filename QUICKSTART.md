# Quickstart

## Prerequisites

- Local stack running
- [Speakeasy CLI](https://www.speakeasy.com/docs/speakeasy-cli/getting-started) installed
- Go installed

## Build the CLI

First export the OpenAPI spec from the running local server (requires local stack running):

```bash
cd packages/backend-api
just openapi > /tmp/openapi.json
```

Then generate and build the CLI:

```bash
cd apimetrics-cli
speakeasy run
go mod tidy
go build -o apimetrics ./cmd/apimetrics
```

## Configure credentials

Create an API key at `http://localhost:8080/settings/api-key`, then run:

```bash
./apimetrics configure
```

Enter your API key when prompted. The project ID is inferred from the key automatically.

## Try it out

```bash
# List browser monitors
./apimetrics browser-monitors list

# Create a browser monitor
./apimetrics browser-monitors create --name 'My Monitor' --url https://example.com/
```

## Local development

Override the server URL to point at your local stack:

```bash
export APIMETRICS_SERVER_URL=http://localhost:8080
```
