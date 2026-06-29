# Quality Guidelines

> Code quality standards for backend development.

---

## Overview

Backend code must be optimized for long-term maintenance. Handlers, services, repositories, and domain code should expose clear contracts and fail explicitly when required data or dependencies are unavailable.

All backend changes must follow [Maintainability Contract](../guides/maintainability-contract.md).

---

## Forbidden Patterns

- Broad error handling that converts failures into empty success responses.
- Silent default values for required request fields, configuration, or persistence results.
- Compatibility branches for API versions or payload shapes that are not explicitly supported.
- Repositories or services returning mock data after a real dependency fails.
- Handler logic that mixes transport parsing, business rules, persistence, and response formatting in one large function.

---

## Required Patterns

- Validate required inputs at the boundary and return clear errors.
- Keep business logic outside HTTP handlers when the behavior grows beyond simple routing.
- Return typed domain errors or explicit status decisions instead of ambiguous booleans.
- Log operational failures at the layer that has enough context to explain them.
- Delete obsolete fallback branches when replacing data contracts or integrations.

---

## Testing Requirements

- Test the normal successful path.
- Test required error paths for boundary validation and dependency failures.
- Do not add tests for speculative fallback behavior unless the fallback is part of a documented contract.

---

## Code Review Checklist

- Does the code fail explicitly when required data is missing?
- Are errors returned or logged where they can be acted on?
- Are fallback branches tied to a documented requirement and covered by focused tests?
- Does the package boundary make future changes cheaper, not harder?
