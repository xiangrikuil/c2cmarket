-- Restore the historical fixed Sub2API service-model multiplier constraint.
-- Date: 2026-07-27
-- Executor: Codex

ALTER TABLE api_service_models
DROP CONSTRAINT IF EXISTS ck_api_service_models_sub2api_multiplier,
ADD CONSTRAINT ck_api_service_models_sub2api_multiplier
CHECK (distribution_system <> 'sub2api' OR merchant_multiplier = 1.0000);
