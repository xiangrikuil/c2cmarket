# Dependency and toolchain hardening implementation plan

## Checklist

- [x] Inspect `frontend/package.json` and `frontend/pnpm-lock.yaml` for `latest` ranges and resolved versions.
- [x] Replace `latest` ranges with explicit existing major/minor-compatible ranges.
- [x] Add frontend `engines` for Node `>=24.11 <25` and pnpm `>=10 <11`.
- [x] Inspect `backend/go.mod`, Dockerfile(s), CI, and README for Go/Node/pnpm/PostgreSQL version references.
- [x] Align Go version references with `backend/go.mod` and document local prerequisites in README.
- [x] Run `pnpm install --frozen-lockfile` in `frontend`.
- [x] Run `pnpm typecheck`, real-mode `pnpm build`, and `pnpm test` in `frontend`.
- [x] Run backend `go test ./...` through Docker Go if native Go is unavailable.
- [x] Run `git diff --check`.

## Validation Results

- `PATH=/Users/lixinjian/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin:$PATH npx --registry=https://registry.npmjs.org pnpm@10 --dir frontend install --frozen-lockfile --config.confirmModulesPurge=false` passed with Node v24.14.0 and pnpm v10.34.4.
- `PATH=/Users/lixinjian/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin:$PATH /Users/lixinjian/.npm/_npx/a1a38f5f0f780954/node_modules/.bin/pnpm --dir frontend typecheck` passed.
- `VITE_API_MODE=real PATH=/Users/lixinjian/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin:$PATH /Users/lixinjian/.npm/_npx/a1a38f5f0f780954/node_modules/.bin/pnpm --dir frontend build` passed; Vite reported third-party Rolldown pure-annotation warnings from `@vueuse/core`.
- `PATH=/Users/lixinjian/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin:$PATH /Users/lixinjian/.npm/_npx/a1a38f5f0f780954/node_modules/.bin/pnpm --dir frontend test` passed.
- `docker run --rm -v /Users/lixinjian/Crypto/c2cmarket:/src -w /src/backend golang:1.26-alpine go test ./...` passed.
- `git diff --check` passed.
- `rg -n '"latest"|specifier: latest' frontend/package.json frontend/pnpm-lock.yaml` found no matches.

## Validation Commands

```bash
pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend typecheck
VITE_API_MODE=real pnpm --dir frontend build
pnpm --dir frontend test
docker run --rm -v /Users/lixinjian/Crypto/c2cmarket:/src -w /src/backend golang:1.26-alpine go test ./...
git diff --check
```

## Risk Points

- `pnpm install --frozen-lockfile` must not rewrite the lockfile. If it fails because package ranges no longer match, update ranges from the lockfile rather than upgrading packages.
- CI, Docker, and README must not drift from `backend/go.mod`.
- Avoid broad dependency modernization; the purpose is repeatability, not upgrades.
