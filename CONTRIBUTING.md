# Contributing

## Development

See `AGENTS.md` for the build, lint, test, and generate commands, and for architecture notes.

## Releasing

Releases are cut by pushing a tag matching `v*` (e.g. `v0.1.0`), which triggers the `Release` GitHub Actions workflow (GoReleaser). That workflow signs release checksums with a repository-held GPG key.

Tags themselves must be signed too:

```sh
git tag -s vX.Y.Z -m "vX.Y.Z"
git push origin vX.Y.Z
```

Signed tags require `tag.gpgsign = true` (and a working signing key) in your git config. This repo previously had `tag.gpgsign` and `commit.gpgsign` disabled locally, overriding the global default — check `git config --local --list` if signing unexpectedly fails.
