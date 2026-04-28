You are an experienced, pragmatic software engineering AI agent. Do not over-engineer a solution when a simple one is possible. Keep edits minimal. If you want an exception to ANY rule, you MUST stop and get permission first.

# AGENTS Guide

## Project Overview

This repository is a **community-maintained, unofficial Terraform provider for [Nayatel Cloud](https://cloud.nayatel.com)**. It lets Terraform manage Nayatel Cloud compute and networking resources through the HashiCorp Terraform Plugin Framework.

Primary goals:
- Manage Nayatel Cloud resources: instances, networks, routers, floating IPs, security groups and attachments, volumes and attachments, and SSH keys.
- Expose data sources for images, flavors, SSH keys, networks, security groups, routers, floating IPs, and volumes.
- Preserve predictable Terraform lifecycle behavior while protecting users from unwanted charges through preview and balance checks before billable operations.

Technology choices:
- Language: **Go** (`go.mod` currently declares Go `1.25`).
- Provider framework: `github.com/hashicorp/terraform-plugin-framework` using protocol v6.
- Testing: Go `testing` plus `github.com/hashicorp/terraform-plugin-testing`.
- Linting/formatting: `golangci-lint` and `gofmt`.
- Documentation generation: `terraform-plugin-docs` via `go generate` in the separate `tools/` module.
- Release: GoReleaser and GitHub Actions.

## Reference

### Important files

- `main.go`: provider server entrypoint; supports `-debug` for debugger-compatible provider runs.
- `internal/provider/provider.go`: provider schema, env-var configuration, client injection, and resource/data source registration.
- `internal/provider/resource_*.go`: Terraform resource schemas and lifecycle implementations.
- `internal/provider/data_sources.go`: all data source implementations.
- `internal/client/client.go`: HTTP client, authentication, token caching, shared request logic, balance checks, and service wiring.
- `internal/client/*.go`: domain-specific Nayatel API services and models (`instances`, `networks`, `floating_ips`, etc.).
- `internal/client/safety_check_test.go`: live API safety-check smoke test that does not create resources but requires credentials.
- `GNUmakefile`: canonical local build/lint/test/generate commands.
- `.golangci.yml`: lint configuration, including depguard bans for Terraform Plugin SDK v2 helper imports.
- `tools/tools.go`: `go:generate` hooks for `terraform fmt -recursive ../examples/` and provider docs generation.
- `.github/workflows/test.yml`: CI build, lint, unit test, and generated-docs verification.
- `.github/workflows/release.yml` and `.goreleaser.yml`: tag-based `v*` release flow and signed GoReleaser artifacts.
- `.github/pull_request_template.md`: required PR sections.

### Directory layout

- `internal/provider/`: Terraform-facing layer. Keep schemas, `types.*` models, diagnostics, plan modifiers, lifecycle methods, imports, and acceptance tests here.
- `internal/client/`: Nayatel API layer. Add or update service methods here before wiring them into provider resources/data sources.
- `docs/`: generated Terraform provider docs. Do not hand-edit as a substitute for schema/example changes; run `make generate`.
- `examples/`: Terraform examples used by docs generation. Keep them valid and run Terraform formatting when touched.
- `test/`: local/manual Terraform configuration and ignored local artifacts.
- `tools/`: separate Go module for docs-generation tooling.

### Architecture notes

- Provider configuration accepts `username`, `password`, `token`, `project_id`, and `base_url`, with environment variable fallbacks: `NAYATEL_USERNAME`, `NAYATEL_PASSWORD`, `NAYATEL_TOKEN`, `NAYATEL_PROJECT_ID`, and `NAYATEL_BASE_URL`.
- `Configure` validates credentials and creates one shared `*client.Client`, which is assigned to both `resp.ResourceData` and `resp.DataSourceData`.
- Username is required even when using token authentication.
- Password login uses a cached JWT when possible; cache files live under `$XDG_CONFIG_HOME/nayatel` or `~/.config/nayatel`.
- Client requests set `Authorization: Bearer <token>` and JSON headers in `Client.Request`.
- `Client.GetProjectID` lazily discovers the first project if no project ID is configured.
- Nayatel API response shapes vary. Existing client list/preview helpers often decode multiple formats; preserve that defensive parsing when adding endpoints.

## Essential commands

Run commands from the repository root unless noted.

### Build and install

```sh
make build
go build -v ./...
make install
```

### Format

```sh
make fmt
terraform fmt -recursive examples/
```

`make fmt` runs `gofmt -s -w -e .`. Terraform example formatting is also run by `make generate`.

### Lint

```sh
make lint
golangci-lint run
```

### Test

```sh
make test
go test -v -cover -timeout=120s -parallel=10 ./...
```

Useful targeted checks:

```sh
go test ./internal/provider -run TestExtractCostFromPreview
go test -v -run TestSafetyChecks ./internal/client/.
```

`TestSafetyChecks` calls live balance/preview APIs and requires `NAYATEL_USERNAME` plus `NAYATEL_TOKEN`; it is intended not to create resources.

Acceptance tests create real resources and may incur costs. Run them only with explicit credentials and cost awareness:

```sh
make testacc
```

### Generate docs/examples

```sh
make generate
```

This runs `cd tools; go generate ./...`, formats `examples/`, and regenerates `docs/`.

### Clean

There is no `make clean` target.

```sh
go clean -testcache
```

If a truly pristine ignored-file cleanup is needed, ask first before running `git clean -fdX`; it removes ignored local state, provider binaries, and Terraform working files.

### Development provider run

```sh
go run . -debug
```

For local Terraform use, build/install the provider and configure Terraform `dev_overrides` as shown in `README.md`.

### Other scripts/workflows

- `find . -type f -name '*.sh'` currently finds no project shell scripts.
- `make` with no target runs `fmt lint install generate`.
- Releases are created by pushing tags matching `v*`; the release workflow expects GPG signing secrets.

## Patterns

- **Provider/resource skeletons:** Follow existing `Metadata`, `Schema`, `Configure`, lifecycle method, and interface assertion patterns (`var _ resource.Resource = &XResource{}`).
- **Diagnostics-first error handling:** In provider code, append diagnostics, check `HasError()`, add Terraform-friendly diagnostics, and return early. Do not panic for user/API errors.
- **Framework models:** Use `types.String`, `types.Int64`, `types.Bool`, etc. with `tfsdk` tags in Terraform-facing models.
- **Plan modifiers:** Preserve `RequiresReplace` and `UseStateForUnknown` behavior unless intentionally changing resource lifecycle semantics.
- **Cost-aware creation:** For billable create/allocation paths, prefer `SafeCreate`/`SafeAllocate` helpers. They preview cost, extract required balance, call `VerifyBalance`, then perform the API operation with retry behavior.
- **Plan-time cost previews:** Existing resources may populate `monthly_cost` in `ModifyPlan`. If previews fail during planning, follow existing behavior: warn with `tflog` instead of making the plan unusable unless the actual create path would be unsafe.
- **Client service wiring:** When adding a service, update the service struct/file, models if needed, `Client` fields, `NewClient` initialization, and provider resource/data-source registration.
- **Import formats:** Keep import formats exact. Examples include simple passthrough IDs for instances/networks/routers/managed security groups/volumes, SSH keys imported by name, floating IPs imported by IP address, and composite IDs such as `instance_id:security_group_name`, `instance_id:floating_ip`, and `volume_id:instance_id`.
- **Acceptance tests:** Use `testAccPreCheck` and `testAccProtoV6ProviderFactories`; keep HCL configs self-contained in helper functions and use `terraform-plugin-testing/helper/resource`.

## Anti-patterns

- **Do not import Terraform Plugin SDK v2 helpers in new code.** `.golangci.yml` depguard denies SDK v2 helper packages; use Plugin Framework and `terraform-plugin-testing` equivalents.
- **Do not bypass charge-safety helpers** for instances, networks, or floating IP allocation paths.
- **Do not run acceptance tests casually.** They require real Nayatel credentials and can create billable resources.
- **Do not commit credentials, tokens, `.tfstate`, `.terraform/`, provider binaries, or other local artifacts.** `.gitignore` excludes common Terraform state and build outputs; keep secrets in environment variables.
- **Do not hand-edit generated docs as the only change** when schema descriptions or examples are the source of truth. Update the source and run `make generate`.
- **Do not loosen pinned GitHub Actions or CI safeguards** without an explicit reason.

## Code style

- Follow idiomatic Go and `gofmt`/`gofmt -s` output.
- Keep edits small and localized; avoid broad refactors unless requested.
- Preserve existing naming patterns (`NewXResource`, `XResourceModel`, `XService`, `SafeCreate`, `SafeAllocate`).
- Preserve MPL-2.0 license/header style in files that already use it; for new files, match nearby package conventions.
- Prefer clear error wrapping with `%w` in client code and concise Terraform diagnostics in provider code.

## Commit and Pull Request Guidelines

### Before committing

1. Run validation relevant to the change:
   - `make fmt`
   - `make lint`
   - `make test`
2. If schemas, docs, generated files, or examples changed, run `make generate` and commit generated diffs.
3. Run `make testacc` only when explicitly needed and when credentials/cost impact are understood.
4. Check `git diff --check` before handing off.

### Commit messages

Git history is mixed (`Bump ...`, `Update ...`, `[COMPLIANCE] ...`, `Implement ...`). For normal hand-written changes, prefer a concise conventional-style subject:

```text
<type>: <imperative summary>
```

Examples: `fix: handle floating IP detach retries`, `docs: refresh provider examples`. Match Dependabot or compliance style when modifying those generated/update streams.

### Pull requests

Use `.github/pull_request_template.md` and fill in:
- **Summary:** what changed and why.
- **Related Issue (optional):** `Fixes #<issue-number>` when applicable.
- **Validation:** commands run and results (`make fmt`, `make lint`, `make test`, `make generate` if needed).
- **Notes for Reviewers:** behavioral changes, especially resource lifecycle, import formats, cost/safety behavior, or breaking changes.
