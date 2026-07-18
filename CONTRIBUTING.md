# Contributing

## Development

See `AGENTS.md` for the build, lint, test, and generate commands, and for architecture notes.

## Signing

All commits and tags in this repo must be signed:

```sh
git commit -S -m "..."
git tag -s vX.Y.Z -m "vX.Y.Z"
```

This requires `commit.gpgsign = true` and `tag.gpgsign = true` (plus a working signing key) in your git config. This repo previously had both disabled locally, overriding the global default — check `git config --local --list` if signing unexpectedly fails or produces unsigned commits/tags.

## Releasing

Releases are cut by pushing a signed tag matching `v*` (e.g. `v0.1.0`), which triggers the `Release` GitHub Actions workflow (GoReleaser):

```sh
git tag -s vX.Y.Z -m "vX.Y.Z"
git push origin vX.Y.Z
```

That workflow additionally signs release checksums with a repository-held GPG key, separate from the git tag signature above.
