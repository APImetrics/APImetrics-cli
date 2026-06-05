# Release Process

This document covers the one-time infrastructure setup required to run the release workflow, and how to cut a release once everything is configured.

## How releases work

Pushing a `v0.x.y` tag triggers the release workflow (`.github/workflows/release.yml`), which:

1. **Linux job** — cross-compiles all platform binaries inside `goreleaser-cross` (Docker), builds archives, and creates a GitHub draft release.
2. **macOS job** — signs and notarizes the Darwin binaries using an Apple Developer certificate, re-archives them, uploads all artifacts to GCS, replaces the macOS assets on the draft release, publishes the release, then updates the Homebrew tap formula.

Snapshot builds (no publish) can be triggered via `workflow_dispatch` with `snapshot: true`.

---

## One-time infrastructure setup

### 1. Google Cloud — Workload Identity Federation

The workflow authenticates to GCP without a stored service account key using Workload Identity Federation (WIF). Set this up once per GCP project.

```bash
PROJECT_ID=your-gcp-project
POOL_NAME=github-actions
PROVIDER_NAME=github
SA_NAME=apimetrics-cli-release
BUCKET=apimetrics-cli
REPO=APImetrics/APImetrics-cli

# Create the workload identity pool
gcloud iam workload-identity-pools create "$POOL_NAME" \
  --project="$PROJECT_ID" \
  --location=global \
  --display-name="GitHub Actions"

# Create the OIDC provider
gcloud iam workload-identity-pools providers create-oidc "$PROVIDER_NAME" \
  --project="$PROJECT_ID" \
  --location=global \
  --workload-identity-pool="$POOL_NAME" \
  --display-name="GitHub" \
  --attribute-mapping="google.subject=assertion.sub,attribute.repository=assertion.repository" \
  --issuer-uri="https://token.actions.githubusercontent.com"

# Create the service account
gcloud iam service-accounts create "$SA_NAME" \
  --project="$PROJECT_ID" \
  --display-name="APImetrics CLI release"

# Grant it write access to the GCS bucket
gsutil iam ch "serviceAccount:${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com:roles/storage.objectAdmin" \
  "gs://$BUCKET"

# Allow the GitHub repo to impersonate the service account
POOL_ID=$(gcloud iam workload-identity-pools describe "$POOL_NAME" \
  --project="$PROJECT_ID" --location=global --format="value(name)")

gcloud iam service-accounts add-iam-policy-binding \
  "${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com" \
  --project="$PROJECT_ID" \
  --role=roles/iam.workloadIdentityUser \
  --member="principalSet://iam.googleapis.com/${POOL_ID}/attribute.repository/${REPO}"
```

Then retrieve the values to store as secrets:

```bash
# Workload identity provider (store as GCP_WORKLOAD_IDENTITY_PROVIDER)
gcloud iam workload-identity-pools providers describe "$PROVIDER_NAME" \
  --project="$PROJECT_ID" \
  --location=global \
  --workload-identity-pool="$POOL_NAME" \
  --format="value(name)"

# Service account email (store as GCP_SERVICE_ACCOUNT)
echo "${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"
```

**Secrets to add to `APImetrics/APImetrics-cli`:**

| Secret | Value |
|--------|-------|
| `GCP_WORKLOAD_IDENTITY_PROVIDER` | Full provider resource name (`projects/.../providers/github`) |
| `GCP_SERVICE_ACCOUNT` | Service account email (`apimetrics-cli-release@….iam.gserviceaccount.com`) |

---

### 2. GitHub App — Homebrew tap access

The workflow uses a GitHub App to push formula updates to `APImetrics/homebrew-tap`. This avoids a long-lived personal access token.

1. Go to **GitHub → APImetrics org settings → Developer settings → GitHub Apps → New GitHub App**.
2. Name it something like `apimetrics-release-bot`.
3. Under **Permissions → Repository permissions**, set **Contents** to `Read & write`. No other permissions needed.
4. Uncheck **Webhooks** (not needed).
5. Set **Where can this GitHub App be installed?** to `Only on this account`.
6. Create the app and note the **App ID**.
7. Under **Private keys**, generate and download a private key (`.pem` file).
8. Go to **Install App** and install it on `APImetrics/homebrew-tap` only.

**Secrets to add to `APImetrics/APImetrics-cli`:**

| Secret | Value |
|--------|-------|
| `GH_APP_ID` | Numeric App ID shown on the app settings page |
| `GH_APP_PRIVATE_KEY` | Full contents of the downloaded `.pem` file |

---

### 3. Apple code signing and notarization

macOS binaries must be signed and notarized with an Apple Developer ID certificate so they run without Gatekeeper warnings.

**Prerequisites:**
- An [Apple Developer Program](https://developer.apple.com/programs/) membership.
- A **Developer ID Application** certificate. Export it as a `.p12` file from Keychain Access (include the private key, set a strong export password).
- An [app-specific password](https://support.apple.com/en-us/102654) for the Apple ID used to notarize.

**Encode the certificate:**

```bash
base64 -i certificate.p12 | pbcopy   # copies base64 to clipboard
```

**Secrets to add to `APImetrics/APImetrics-cli`:**

| Secret | Value |
|--------|-------|
| `APPLE_CERT_P12` | Base64-encoded `.p12` certificate (see above) |
| `APPLE_CERT_PASSWORD` | Password set when exporting the `.p12` |
| `KEYCHAIN_PASSWORD` | Any strong random string (used for the temporary CI keychain) |
| `APPLE_SIGNING_IDENTITY` | Certificate common name, e.g. `Developer ID Application: APImetrics Inc (XXXXXXXXXX)` |
| `APPLE_ID` | Apple ID email used for notarization |
| `APPLE_ID_PASSWORD` | App-specific password for that Apple ID |
| `APPLE_TEAM_ID` | 10-character Apple Developer Team ID |

---

### 4. Homebrew tap repository

The Homebrew formula is pushed to `APImetrics/homebrew-tap`. Ensure:

- The repository exists.
- A `Formula/` directory exists in the repository root (create it with a `.gitkeep` if needed — GoReleaser will create `Formula/apimetrics.rb` on first release).
- The GitHub App from step 2 is installed on this repository.

---

## Complete secrets reference

All secrets required on `APImetrics/APImetrics-cli`:

| Secret | Purpose |
|--------|---------|
| `GCP_WORKLOAD_IDENTITY_PROVIDER` | WIF provider for GCS uploads |
| `GCP_SERVICE_ACCOUNT` | GCP service account email for GCS uploads |
| `GH_APP_ID` | GitHub App ID for Homebrew tap access |
| `GH_APP_PRIVATE_KEY` | GitHub App private key for Homebrew tap access |
| `APPLE_CERT_P12` | Base64 Apple Developer ID certificate |
| `APPLE_CERT_PASSWORD` | Password for the p12 certificate |
| `KEYCHAIN_PASSWORD` | Temporary CI keychain password |
| `APPLE_SIGNING_IDENTITY` | Apple signing identity string |
| `APPLE_ID` | Apple ID for notarization |
| `APPLE_ID_PASSWORD` | App-specific password for notarization |
| `APPLE_TEAM_ID` | Apple Developer Team ID |

---

## Cutting a release

Once all secrets are configured:

```bash
git tag v0.x.y
git push origin v0.x.y
```

Monitor progress in the [Actions tab](https://github.com/APImetrics/APImetrics-cli/actions). The full pipeline (Linux build + macOS sign/notarize) takes approximately 20–30 minutes.

### Snapshot builds (no publish)

To build all artifacts without publishing anywhere:

1. Go to **Actions → Release → Run workflow**.
2. Check **Run goreleaser in --snapshot mode**.
3. Run from any branch.

### Tag pattern note

The workflow only triggers on `v0.*` tags. This is intentional during initial development to prevent accidental production releases. Update the tag pattern in `.github/workflows/release.yml` to `v*` when ready to ship v1.0.
