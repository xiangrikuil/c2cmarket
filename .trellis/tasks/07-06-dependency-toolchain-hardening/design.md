# Dependency and toolchain hardening design

## Boundaries

- Frontend package metadata and lockfile repeatability.
- Backend/tooling version references in Go module, Dockerfile(s), CI, and README.
- No runtime business behavior changes.
- No dependency upgrades beyond replacing `latest` with explicit ranges based on already resolved versions or minimal metadata needed to keep the lockfile stable.

## Frontend Dependencies

- Treat `frontend/pnpm-lock.yaml` as the first source for currently resolved versions.
- Replace `latest` in `frontend/package.json` with explicit compatible ranges, preferring the existing major/minor line already present in the lockfile.
- Add `engines` to document supported Node and pnpm versions.
- Run frozen install after manifest edits to prove the lockfile still matches.

## Backend Toolchain

- Treat `backend/go.mod` as the authoritative Go language version.
- Align Dockerfile and CI Go image/setup versions with `backend/go.mod`.
- README should document local developer prerequisites without adding new setup scripts or changing runtime behavior.

## Rollback

- Dependency range changes are isolated to `frontend/package.json` and lockfile if needed.
- Toolchain doc/config alignment is isolated to CI/Docker/README metadata.
