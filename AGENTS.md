You are an experienced, pragmatic software engineering AI agent. Do not over-engineer a solution when a simple one is possible. Keep edits minimal. If you want an exception to ANY rule, you MUST stop and get permission first.

# AGENTS Guide

## Project Overview

This repository is a **Terraform provider for [Nayatel Cloud](https://cloud.nayatel.com)**. It implements resources and data sources using the HashiCorp **Terraform Plugin Framework** (protocol v6), plus a custom Go API client for Nayatel Cloud endpoints.

Primary goals:
- Manage core cloud resources (instances, networks, routers, floating IPs, security groups, volumes, SSH keys).
- Expose Terraform-native resources/data sources with predictable lifecycle behavior.
- Add safety checks (preview + balance verification) before operations that may incur charges.

Tech stack:
- Language: **Go** (module in root, tools module in `tools/`)
- Terraform provider framework: `terraform-plugin-framework`
- Testing: Go `testing`, `terraform-plugin-testing`
- Linting: `golangci-lint`
- Docs generation: `terraform-plugin-docs` via `go generate` in `tools/`
- Release: `goreleaser` + GitHub Actions

## Reference

### Important files
- `main.go`: provider server entrypoint, supports `-debug` flag.
- `internal/provider/provider.go`: provider schema/config + registration of all resources/data sources.
- `internal/provider/resource_*.go`: Terraform resource lifecycle implementations.
- `internal/provider/data_sources.go`: all data source implementations.
- `internal/client/client.go`: HTTP client, auth/token cache, shared request logic.
- `internal/client/*.go`: per-domain API services (instances, networks, floating IPs, etc.).
- `GNUmakefile`: canonical local commands.
- `.golangci.yml`: enforced lint rules (including depguard restrictions).
- `tools/tools.go`: `go:generate` hooks for headers, Terraform fmt (examples), and docs generation.
- `.github/workflows/test.yml`: CI build/lint/generate checks and acceptance test job.
- `.github/pull_request_template.md`: required PR description sections.

### Directory layout
- `internal/provider/`: Terraform-facing layer (schema/state/lifecycle/import/plan modifiers).
- `internal/client/`: Nayatel API layer.
- `docs/`: generated provider docs.
- `examples/`: Terraform usage examples.
- `test/`: local Terraform config for manual verification.
- `tools/`: separate Go module for generation tooling.

### Architecture notes
- Provider configuration creates one shared `*client.Client` and injects it into resources/data sources.
- Resource create paths frequently use `SafeCreate`/`SafeAllocate` helpers to avoid accidental charge-causing calls.
- Some imports use composite IDs (e.g., `instance_id:security_group_name`, `volume_id:instance_id`). Keep format handling exact.

## Essential commands

Run from repository root unless noted.

### Build
- `make build`
- `go build -v ./...`

### Format
- `make fmt` (Go code)
- `terraform fmt -recursive examples/` (Terraform examples; also run by generation flow)

### Lint
- `make lint`
- `golangci-lint run`

### Test
- `make test` (unit/integration-style Go tests)
- `go test -v -cover -timeout=120s -parallel=10 ./...`
- `make testacc` (real acceptance tests; requires credentials + may incur cost)

### Clean
No dedicated `make clean` target exists.
- Preferred lightweight cleanup: `go clean -testcache`
- If you truly need a pristine tree: `git clean -fdX` (dangerous; removes ignored files)

### Development server / provider debug run
- `go run . -debug`
- Or install local binary: `make install` then use Terraform `dev_overrides` from `README.md`.

### Other important scripts/workflows
- `make generate` (runs `cd tools; go generate ./...`)
- `find . -type f -name '*.sh'` currently returns no project shell scripts.
- `make` (default) runs: `fmt lint install generate`

## Patterns

- **Resource/data source skeleton**: keep `Metadata`, `Schema`, `Configure`, and lifecycle methods consistent with existing files.
- **Diagnostics-first error handling** in provider layer: add Terraform diagnostics, return early on errors.
- **Plan modifiers** are widely used (`RequiresReplace`, `UseStateForUnknown`) — preserve behavior unless intentionally changing lifecycle semantics.
- **Cost-aware creation pattern**: prefer safe client methods (`SafeCreate`, `SafeAllocate`) over raw create/allocate calls for billable resources.
- **Acceptance tests**: use `testAccPreCheck` and `testAccProtoV6ProviderFactories`; keep configs self-contained in helper functions.

## Anti-patterns

- **Do not import Terraform Plugin SDK v2 helper packages** in new code. `.golangci.yml` depguard explicitly denies these; use Plugin Framework / `terraform-plugin-testing` equivalents.
- **Do not bypass CI safeguards**: keep workflow checks, pinned action SHAs, and generation verification intact.
- **Do not bypass charge-safety checks** for instance/network/floating IP creation paths.
- **Do not commit credentials/tokens** in examples/tests/fixtures; use environment variables (`NAYATEL_*`) for local and CI runs.

## Code style

- Follow existing Go style + `gofmt` output.
- Keep license headers in source files consistent with the repository's MPL-2.0 licensing.
- Prefer small, targeted edits over broad refactors.
- Use existing naming patterns (`ResourceModel`, `NewXResource`, service types in `client`).

## Commit and Pull Request Guidelines

### Before committing
1. Run formatting/lint/tests relevant to your change:
   - `make fmt`
   - `make lint`
   - `make test`
2. If schema/docs/examples changed, run `make generate` and commit generated diffs.
3. For acceptance-impacting changes, run `make testacc` only with explicit credentials and cost awareness.

### Commit messages
- Preferred convention: `<type>: <short summary>` (e.g., `fix: handle floating IP detach retries`).
- Keep subject imperative and specific.
- If following existing repo patterns (e.g., dependency bumps), match surrounding style.

### Pull requests
- Use `.github/pull_request_template.md` sections:
  - Related Issue (`Fixes #...`)
  - Description with rationale
  - Rollback plan checklist
  - Security controls impact statement
- Include evidence of validation (commands + outcomes).
- Call out any behavior changes affecting resource lifecycle, import formats, or cost/safety checks.
