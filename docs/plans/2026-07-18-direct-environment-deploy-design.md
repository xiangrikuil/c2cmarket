# Direct GitHub Environment Deployment Design

Date: 2026-07-18

Executor: Codex

## Outcome

The reusable backend workflow publishes immutable GHCR images only. Direct
jobs in `.github/workflows/ci.yml` bind the `staging` and `production`
environments, consume their SSH secrets, and deploy the tested image SHA.

## Options considered

1. Pass environment secrets into the reusable workflow. The caller job cannot
   bind an environment while using `jobs.<id>.uses`, so it cannot reliably own
   or forward those secrets.
2. Replace environment secrets with repository secrets. This would make secret
   passing simple but remove staging/production isolation and weaken the
   production reviewer boundary.
3. Keep image publishing reusable and move deployment to direct top-level jobs.
   This preserves isolation and approvals while retaining one shared deployment
   step sequence through a YAML anchor. This option is selected.

## Bug analysis

### Root cause category

- Category B, cross-layer contract: the contract between a caller workflow,
  called workflow, and GitHub Environment secret resolution was assumed rather
  than proven by a real deployment.
- Category E, implicit assumption: a recorded `staging` deployment was treated
  as proof that its environment secrets were injected.

### Why the first fix failed

The first fix changed dynamic environment names to literal names but left the
deployment jobs inside the reusable workflow. It addressed name resolution,
not the caller/callee secret boundary. GitHub continued to record the expected
environment while all four VPS variables resolved to empty strings.

### Prevention

- Architecture: reusable workflows may publish images but must not own VPS
  deployment jobs or read environment-scoped SSH secrets.
- Tests: assert direct deployment jobs and secret references in `ci.yml`, and
  assert their absence from `release-backend.yml`.
- Operations: validate a deployment with the actual GitHub Environment after
  merge; YAML parsing and unit tests cannot emulate server-side secret
  injection.

## Execution and validation

The staging and production deployment jobs each depend on their matching image
publish job, use the immutable `${{ github.sha }}` image tag, and retain separate
concurrency groups. Production continues to wait on its Environment reviewer.
Validation covers workflow YAML parsing, deployment configuration tests, VPS
release script tests, project CI checks, and the first real staging deployment
after merge.
