# Build targets for each environment configuration. The environment is
# selected by Go build tags (see config_*.go); each build bakes in its own
# command name so the binaries can be installed side by side.

# Build every configuration
build: prod dev qc qc-stable beta

# Production (client.apimetrics.io)
prod:
    go build -tags prod -o apimetrics .

# Local development (localhost:8080)
dev:
    go build -tags dev -o apimetrics-dev .

# QC (qc-client.apimetrics.io) — the default when no tag is given
qc:
    go build -o apimetrics-qc .

# QC stable (qc-stable.apimetrics.io)
qc-stable:
    go build -tags qcstable -o apimetrics-qc-stable .

# Beta (beta-client.apimetrics.io, production Auth0)
beta:
    go build -tags beta -o apimetrics-beta .

test:
    go test ./...

clean:
    rm -f apimetrics apimetrics-dev apimetrics-qc apimetrics-qc-stable apimetrics-beta
