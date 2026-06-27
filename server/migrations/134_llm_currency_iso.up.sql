-- Switch llm_model.currency from display symbols to ISO currency codes.
-- Migration 133 shipped with symbol defaults (¥/$/€); this normalizes
-- existing rows and the column default to CNY/USD/EUR. Amounts are still
-- rendered with the matching symbol (¥/$/€) in the UI.
UPDATE llm_model SET currency = 'CNY' WHERE currency = '¥';
UPDATE llm_model SET currency = 'USD' WHERE currency = '$';
UPDATE llm_model SET currency = 'EUR' WHERE currency = '€';

ALTER TABLE llm_model ALTER COLUMN currency SET DEFAULT 'CNY';
COMMENT ON COLUMN llm_model.currency IS 'Pricing currency ISO code: CNY, USD, EUR. Default CNY. Amounts are displayed with the matching symbol (¥/$/€).';
