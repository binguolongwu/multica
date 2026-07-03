-- 144 down: no-op.
-- These are instructions-only rewrites of existing template rows. Rolling back
-- would discard the improved contracts with no replacement; the prior
-- instructions are preserved in git history if a restore is ever needed.
SELECT 1;
