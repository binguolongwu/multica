-- 136_agent_template.down.sql

DROP TABLE IF EXISTS agent_template;
ALTER TABLE "user" DROP COLUMN IF EXISTS platform_admin;
