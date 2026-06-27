-- Add per-model pricing fields to llm_model.
-- Prices are expressed per million tokens, in the unit of `currency`.
-- Used by the LLM settings UI to display/edit model pricing; the static
-- metrics pricing map (internal/metrics/pricing.go) is unaffected.
ALTER TABLE llm_model
    ADD COLUMN currency     TEXT NOT NULL DEFAULT 'CNY',
    ADD COLUMN input_price  DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN output_price DOUBLE PRECISION NOT NULL DEFAULT 0;

COMMENT ON COLUMN llm_model.currency IS 'Pricing currency ISO code: CNY, USD, EUR. Default CNY. Amounts are displayed with the matching symbol (¥/$/€).';
COMMENT ON COLUMN llm_model.input_price IS 'Input price per million tokens (in currency).';
COMMENT ON COLUMN llm_model.output_price IS 'Output price per million tokens (in currency).';
