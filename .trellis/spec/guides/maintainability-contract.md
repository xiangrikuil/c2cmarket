# Maintainability Contract

> Hard requirements for every change in this project.

日期：2026-06-17
执行者：Codex

---

## Core Requirements

This project is built for long-term maintenance. Every implementation must optimize for clear ownership, stable structure, readable code, and easy future modification.

Code should be explicit about the real business path. Do not bury the main logic under excessive fallbacks, broad compatibility branches, silent default values, or speculative recovery paths.

---

## Hard Rules

- Prefer boring, direct implementations over clever abstractions.
- Keep modules small and named by business purpose or technical responsibility.
- Add abstractions only when they remove proven duplication or match an existing project pattern.
- Make invalid states visible early with clear errors instead of silently continuing with guessed data.
- Do not add fallback behavior unless there is a concrete product requirement or documented external dependency instability.
- Do not catch broad errors only to return empty data, default success, or a vague generic state.
- Do not introduce compatibility layers for versions, APIs, or data shapes that this project does not actively support.
- Do not hide failed network, parsing, storage, or API operations behind mock data in production code.
- Do not add "just in case" branches without a test proving the branch is required.

---

## Acceptable Fallbacks

Fallbacks are allowed only when all of the following are true:

- The failure mode is expected in normal use.
- The fallback behavior is visible to the caller or user.
- The fallback preserves correctness rather than merely hiding the problem.
- The code has a focused test for the fallback path.

---

## Review Checklist

- Can a new maintainer understand the main path without reading unrelated fallback branches?
- Does each fallback have a real reason documented in code, tests, or product behavior?
- Are errors surfaced at the correct layer?
- Would removing a fallback make a hidden bug visible? If yes, remove or redesign it.
- Does this change reduce future maintenance cost instead of moving complexity elsewhere?

## External Proxy Boundary Diagnosis

When a browser reports an application-layer error for a request that crosses a CDN, Tunnel, reverse proxy, or access gateway, inspect the raw public response before changing application code:

- Compare the local origin response with the public response using the same method and `Origin` header.
- Treat Cloudflare HTTP `530` as a connector/Tunnel failure. Its generated error body does not carry the Go backend's CORS headers, so browsers may misleadingly report it as a CORS violation.
- If the local backend returns the expected CORS headers but the public endpoint returns a gateway status, repair the proxy boundary first; do not broaden the backend allowlist.
- Verify recovery without a temporary foreground process. Stop the diagnostic connector and prove the persistent service still serves health, readiness, credentialed GET, and OPTIONS requests.
- For this project's Mac-hosted Tunnel, repeated QUIC inactivity timeouts with a passing TCP 7844 pre-check require the documented `http2` transport and LaunchAgent restart procedure.

## LaunchAgent Script Interpreter Contract

When a LaunchAgent invokes a repository script through an explicit shell in
`ProgramArguments`, that shell must match the script's syntax and shebang. In
particular, invoke `scripts/backup-production-postgres.sh` with `/bin/bash`;
running it as `/bin/zsh` leaves Bash-only variables such as `BASH_SOURCE`
undefined even though the script itself declares a Bash shebang.

`plutil -lint` validates only the plist structure. After installing or changing
a LaunchAgent, also load or kick-start the installed job and verify all of the
following: the resolved program/arguments, `last exit code = 0`, no new stderr
content, and the expected external side effect (for the production backup job,
both the `.dump` and `.sha256` objects must exist in R2).
