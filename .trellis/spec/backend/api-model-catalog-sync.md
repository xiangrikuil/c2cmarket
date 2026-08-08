# API Model Catalog Pricing Sync And Activation

Date: 2026-08-08
Author: Codex

## Scenario: Administrator-Reviewed models.dev Catalog Sync

### 1. Scope / Trigger

Use this contract when changing the API model catalog, models.dev ingestion, model price versions, model activation, the administrator model page, or the public model picker.

The integration belongs to the existing `catalog` module and `/admin/api-models` page. models.dev is a candidate-data source for an explicit administrator workflow, never an authoritative process that can automatically enable, disable, delete, or overwrite local models.

### 2. Signatures

HTTP endpoints:

```text
POST /api/v1/admin/api-models/models-dev/preview
POST /api/v1/admin/api-models/models-dev/apply
POST /api/v1/admin/api-models/bulk-status
GET  /api/v1/api-models
```

Backend boundaries:

```go
type Source interface {
    Fetch(ctx context.Context) (modelsdev.Catalog, error)
}

PreviewAPIModelSync(ctx, user, APIModelSyncPreviewInput) (APIModelSyncPreview, *domain.AppError)
ApplyAPIModelSyncWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion) (idempotency.Completion, *domain.AppError)
SetAPIModelsActiveWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion) (idempotency.Completion, *domain.AppError)
```

Persistence continues to use `api_model_providers`, `api_model_catalog`, `api_model_price_versions`, and the existing idempotency records. Do not add a sync staging table unless the product contract explicitly changes.

### 3. Contracts

- The backend fetches only the fixed `https://models.dev/api.json` endpoint through the bounded outbound HTTP client. The client timeout is 15 seconds, the decoded body is limited to 16 MiB, redirects are rejected, and trailing JSON is invalid.
- The first supported provider allowlist is `openai`, `anthropic`, `google`, and `perplexity`. Requests carry local provider IDs; the backend resolves and validates their provider codes.
- Keep the exact models.dev model `id` as `modelKey`, including lowercase punctuation such as `gpt-4.1-mini`. Do not derive it from a display name and do not append `/v1` or any other suffix.
- Import candidates must accept text input and produce text output. Embedding, image-output, audio, Realtime, moderation, and video models are excluded from the supported candidate set.
- Map `cost.input`, `cost.cache_read`, and `cost.output` to the existing per-million-token price columns. Map text, image-input/attachment, and reasoning metadata to `text`, `vision`, and `reasoning` in the catalog's canonical capability order.
- Preview is read-only and returns `new`, `price_changed`, `unchanged`, `source_missing`, and `unavailable`. A missing provider or local model is informational and must not mutate local state.
- New rows are selected for import by the frontend but default to `active=false`. Activation is a separate explicit choice. Existing price updates preserve provider, capabilities, sort order, and active state.
- Apply accepts only `new` and `price_changed` selections. The stable fingerprint covers status, provider identity, exact model key, canonical capabilities, source URL/version, prices, and local model/price-version IDs; `active` is intentionally excluded so an administrator can choose activation after preview.
- Apply locks and validates every selected row before writing, rolls the current price version forward, and completes idempotency in one PostgreSQL transaction. Any conflict rolls back the whole batch.
- Bulk status validates and locks the complete model set before one atomic update. The public catalog continues to return only rows where both model and provider are active.
- Real frontend mode must call these endpoints and surface failures. Mock mode must reproduce preview side-effect freedom, fingerprint validation, atomic apply, active-state preservation, and public-catalog invalidation; it must not silently turn a real backend failure into mock success.
- On narrow viewports, the sync dialog itself must remain within the viewport. Apply `min-w-0` through grid/content ancestors and confine the wide comparison table to its own `overflow-x-auto` container; the dialog must not become the horizontal scroller.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| Missing or unsupported provider IDs | `422 VALIDATION_FAILED` with `providerIds` field detail |
| Non-administrator session | `403 PERMISSION_DENIED` |
| models.dev timeout, HTTP failure, invalid top-level JSON, or oversized response | `502 EXTERNAL_SOURCE_UNAVAILABLE`; no catalog writes |
| Unsupported external model type | Exclude it from importable candidates; never write it |
| Invalid model key or price for an otherwise relevant record | `unavailable` preview item with a stable reason code |
| Apply item has a changed payload but an old fingerprint | `422 VALIDATION_FAILED`; no catalog writes |
| Local model or current price version changed after preview | `412 VERSION_CONFLICT`; roll back the batch |
| New model key is now occupied | `412 VERSION_CONFLICT`; roll back the batch |
| Apply or bulk request repeats the same idempotency key and request hash | Replay the stored completion; do not create another price version |
| Bulk request contains an invalid or missing model ID | Validation/not-found error; update no models |

### 5. Good / Base / Bad Cases

- Good: preview four official providers, import selected new models as inactive, explicitly activate one new model, confirm selected price changes, and retain the existing active state of updated models.
- Base: close or cancel preview, receive `source_missing`, or receive an unchanged catalog. Local models, price versions, and activation remain untouched.
- Bad: write during preview, import all providers, normalize `gpt-4.1-mini` into a display label, auto-enable new rows, update only the first valid row of a conflicting batch, or let the 920px comparison table widen a 390px dialog.

### 6. Tests Required

- models.dev client tests assert valid public response shape, HTTP failure, malformed/trailing JSON, response-size limit, and client timeout classification.
- Server tests assert preview status counts, exact model keys, unavailable/source-missing records, tampered fingerprint rejection, atomic create plus price update, default inactive state, existing active-state preservation, idempotency replay, bulk activation, public visibility, and external-source Problem Details.
- PostgreSQL coverage must assert row locking, old price-version closure, new current version creation, whole-batch rollback, stale version conflict, and idempotency completion in the business transaction.
- Frontend adapter tests assert preview has no writes, new imports remain hidden from the public catalog until enabled, price updates preserve active state, stale/tampered selections fail, and bulk status refreshes public/admin queries.
- Run `go test ./...`, frontend Vitest, Nuxt typecheck/build, OpenAPI route/type drift checks, and browser QA at desktop and mobile widths.
- Browser QA must inspect all five preview tabs, initial selection defaults, the activation switch, price comparison, apply result, batch status, horizontal table scrolling, reachable dialog footer, and absence of console errors.

### 7. Wrong vs Correct

#### Wrong

```text
models.dev fetch -> write every returned model -> enable it -> public seller picker
```

```vue
<DialogContent>
  <table class="min-w-[920px]">...</table>
</DialogContent>
```
The first flow bypasses administrator review and local catalog ownership. The second lets grid min-content width make the whole mobile dialog scroll horizontally.

#### Correct

```text
fixed bounded fetch -> read-only classified preview -> explicit selections
  -> fingerprint and local-version validation -> one atomic apply
  -> new rows inactive unless explicitly enabled -> active public catalog only
```

```vue
<DialogContent class="min-w-0 overflow-x-hidden">
  <section class="min-w-0">
    <div class="w-full max-w-full overflow-x-auto">
      <table class="min-w-[920px]">...</table>
    </div>
  </section>
</DialogContent>
```
