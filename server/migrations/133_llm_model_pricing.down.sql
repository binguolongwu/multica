ALTER TABLE llm_model
    DROP COLUMN IF EXISTS output_price,
    DROP COLUMN IF EXISTS input_price,
    DROP COLUMN IF EXISTS currency;
