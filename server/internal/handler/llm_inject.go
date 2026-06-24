package handler

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// autoInjectLLMEnv injects the LLM provider's api_key and api_base_url into
// customEnv when the given model matches a server-side catalog entry. Keys
// that are already present in customEnv are never overwritten — the user's
// explicit configuration always takes precedence.
func (h *Handler) autoInjectLLMEnv(ctx context.Context, model string, customEnv map[string]string) {
	if model == "" {
		return
	}
	provider, err := h.Queries.GetLLMProviderByModelCode(ctx, model)
	if err != nil {
		return // model not in catalog, nothing to inject
	}
	if customEnv == nil {
		customEnv = map[string]string{}
	}
	if provider.EnvVarApiKey != "" && provider.ApiKey != "" {
		if _, exists := customEnv[provider.EnvVarApiKey]; !exists {
			customEnv[provider.EnvVarApiKey] = provider.ApiKey
		}
	}
	if provider.EnvVarBaseUrl != "" && provider.ApiBaseUrl != "" {
		if _, exists := customEnv[provider.EnvVarBaseUrl]; !exists {
			customEnv[provider.EnvVarBaseUrl] = provider.ApiBaseUrl
		}
	}
}

// injectLLMEnvIntoAgent updates an existing agent's custom_env with the
// LLM provider credentials from the catalog. Used by UpdateAgent when the
// model field changes. Existing custom_env keys are never overwritten.
func (h *Handler) injectLLMEnvIntoAgent(ctx context.Context, agentID pgtype.UUID, model string) {
	provider, err := h.Queries.GetLLMProviderByModelCode(ctx, model)
	if err != nil {
		return
	}
	// Load current agent to get existing custom_env for merge.
	agent, err := h.Queries.GetAgent(ctx, agentID)
	if err != nil {
		slog.Warn("llm: failed to load agent for env injection", "agent_id", uuidToString(agentID), "error", err)
		return
	}
	existing := unmarshalCustomEnv(agent)
	if existing == nil {
		existing = map[string]string{}
	}
	inject := false
	if provider.EnvVarApiKey != "" && provider.ApiKey != "" {
		if _, exists := existing[provider.EnvVarApiKey]; !exists {
			existing[provider.EnvVarApiKey] = provider.ApiKey
			inject = true
		}
	}
	if provider.EnvVarBaseUrl != "" && provider.ApiBaseUrl != "" {
		if _, exists := existing[provider.EnvVarBaseUrl]; !exists {
			existing[provider.EnvVarBaseUrl] = provider.ApiBaseUrl
			inject = true
		}
	}
	if !inject {
		return
	}
	raw, err := json.Marshal(existing)
	if err != nil {
		return
	}
	if _, err := h.Queries.UpdateAgentCustomEnv(ctx, db.UpdateAgentCustomEnvParams{
		ID:        agent.ID,
		CustomEnv: raw,
	}); err != nil {
		slog.Warn("llm: failed to inject custom_env", "agent_id", agentID, "error", err)
	}
}
