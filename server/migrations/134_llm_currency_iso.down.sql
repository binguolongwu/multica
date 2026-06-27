-- Revert currency codes back to display symbols and restore the symbol default.
ALTER TABLE llm_model ALTER COLUMN currency SET DEFAULT '¥';
UPDATE llm_model SET currency = '¥' WHERE currency = 'CNY';
UPDATE llm_model SET currency = '$' WHERE currency = 'USD';
UPDATE llm_model SET currency = '€' WHERE currency = 'EUR';
COMMENT ON COLUMN llm_model.currency IS 'Pricing currency symbol: ¥, $, €. Default ¥.';
