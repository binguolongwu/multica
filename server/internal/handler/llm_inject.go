package handler

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (h *Handler) autoInjectLLMEnv(ctx context.Context, workspaceID, model string, customEnv map[string]string) {
	if model == "" || workspaceID == "" {
		return
	}
	provider, err := h.Queries.GetLLMProviderByModelCode(ctx, db.GetLLMProviderByModelCodeParams{
		ModelCode:   model,
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		return
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

func (h *Handler) injectLLMEnvIntoAgent(ctx context.Context, agentID pgtype.UUID, workspaceID, model string) {
	provider, err := h.Queries.GetLLMProviderByModelCode(ctx, db.GetLLMProviderByModelCodeParams{
		ModelCode:   model,
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		return
	}
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
		slog.Warn("llm: failed to inject custom_env", "agent_id", uuidToString(agentID), "error", err)
	}
}
