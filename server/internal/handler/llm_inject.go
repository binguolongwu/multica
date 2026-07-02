package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// autoInjectLLMEnv looks up the LLM provider endpoint matching the given
// model_code and runtime's protocol_family, then injects the provider's
// api_key and the endpoint's api_base_url into the agent's custom_env.
//
// The env var NAMES come from runtime_protocol_map (DB-driven, replaces the
// old hardcoded llmRuntimeEnvVars map). The endpoint is selected by matching
// the runtime's api_type to the provider's endpoint api_type.
//
// Existing keys are never overwritten. The (possibly newly-allocated) map
// is returned because reassigning the map parameter inside the function
// would not propagate back to the caller.
func (h *Handler) autoInjectLLMEnv(ctx context.Context, workspaceID, model, runtimeProvider string, customEnv map[string]string) map[string]string {
	if model == "" || workspaceID == "" {
		return customEnv
	}
	result, err := h.Queries.GetLLMEndpointForInjection(ctx, db.GetLLMEndpointForInjectionParams{
		ModelCode:      model,
		WorkspaceID:    parseUUID(workspaceID),
		ProtocolFamily: runtimeProvider,
	})
	if err != nil {
		return customEnv
	}
	if customEnv == nil {
		customEnv = map[string]string{}
	}
	if result.EnvVarApiKey != "" && result.ApiKey != "" {
		if _, exists := customEnv[result.EnvVarApiKey]; !exists {
			customEnv[result.EnvVarApiKey] = result.ApiKey
		}
	}
	if result.EnvVarBaseUrl != "" && result.ApiBaseUrl != "" {
		if _, exists := customEnv[result.EnvVarBaseUrl]; !exists {
			customEnv[result.EnvVarBaseUrl] = result.ApiBaseUrl
		}
	}
	return customEnv
}

// injectLLMEnvIntoAgent is the UpdateAgent counterpart of autoInjectLLMEnv:
// it re-injects the provider creds (under the runtime's env var names) when
// the model is changed. It resolves the runtime provider from the agent's
// runtime_id so the caller does not need to pass it.
func (h *Handler) injectLLMEnvIntoAgent(ctx context.Context, agentID pgtype.UUID, workspaceID, model string) {
	agent, err := h.Queries.GetAgent(ctx, agentID)
	if err != nil {
		slog.Warn("llm: failed to load agent for env injection", "agent_id", uuidToString(agentID), "error", err)
		return
	}
	runtimeProvider := ""
	if agent.RuntimeID.Valid {
		if rt, err := h.Queries.GetAgentRuntimeForWorkspace(ctx, db.GetAgentRuntimeForWorkspaceParams{
			ID:          agent.RuntimeID,
			WorkspaceID: agent.WorkspaceID,
		}); err == nil {
			runtimeProvider = rt.Provider
		}
	}
	result, err := h.Queries.GetLLMEndpointForInjection(ctx, db.GetLLMEndpointForInjectionParams{
		ModelCode:      model,
		WorkspaceID:    agent.WorkspaceID,
		ProtocolFamily: runtimeProvider,
	})
	if err != nil {
		return
	}
	existing := unmarshalCustomEnv(agent)
	if existing == nil {
		existing = map[string]string{}
	}
	inject := false
	if result.EnvVarApiKey != "" && result.ApiKey != "" {
		if _, exists := existing[result.EnvVarApiKey]; !exists {
			existing[result.EnvVarApiKey] = result.ApiKey
			inject = true
		}
	}
	if result.EnvVarBaseUrl != "" && result.ApiBaseUrl != "" {
		if _, exists := existing[result.EnvVarBaseUrl]; !exists {
			existing[result.EnvVarBaseUrl] = result.ApiBaseUrl
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

// resolveLLMEnvRefs substitutes LLM credential references in an agent's
// custom_env values with concrete values sourced from the LLM provider matched
// by the agent's model_code. Supported references:
//
//	${provider.api_key}      -> matched provider's api_key
//	${provider.api_base_url} -> matched provider's first active endpoint's base_url
//	${model.code}            -> the agent's model string itself
//
// Literal values (those not containing "${") pass through unchanged, so an
// agent whose custom_env holds only literal values pays nothing. When the
// model matches no provider in the catalog, ${provider.*} references resolve to
// an empty string (a debug log is emitted) while ${model.code} still resolves.
// Unknown ${...} tokens are left untouched.
//
// This function does not have runtime context, so it cannot select the
// endpoint by protocol_family. It falls back to the first active endpoint.
// Called at task-claim time so the daemon receives concrete credential values.
func (h *Handler) resolveLLMEnvRefs(ctx context.Context, customEnv map[string]string, workspaceID, model string) map[string]string {
	if len(customEnv) == 0 || model == "" {
		return customEnv
	}
	hasRef := false
	for _, v := range customEnv {
		if strings.Contains(v, "${") {
			hasRef = true
			break
		}
	}
	if !hasRef {
		return customEnv
	}

	resolved := make(map[string]string, len(customEnv))
	for k, v := range customEnv {
		resolved[k] = strings.ReplaceAll(v, "${model.code}", model)
	}

	needProvider := false
	for _, v := range resolved {
		if strings.Contains(v, "${provider.") {
			needProvider = true
			break
		}
	}
	if !needProvider {
		return resolved
	}

	var apiKey, apiBaseURL string
	if provider, err := h.Queries.GetLLMProviderByModelCode(ctx, db.GetLLMProviderByModelCodeParams{
		ModelCode:   model,
		WorkspaceID: parseUUID(workspaceID),
	}); err == nil {
		apiKey = provider.ApiKey
		if endpoints, eerr := h.Queries.ListLLMProviderEndpoints(ctx, db.ListLLMProviderEndpointsParams{
			ProviderID:  provider.ID,
			WorkspaceID: parseUUID(workspaceID),
		}); eerr == nil && len(endpoints) > 0 {
			apiBaseURL = endpoints[0].ApiBaseUrl
		}
	} else {
		slog.Debug("llm env resolve: no provider for model; ${provider.*} refs resolve empty",
			"model", model, "workspace_id", workspaceID, "error", err)
	}
	for k, v := range resolved {
		if !strings.Contains(v, "${provider.") {
			continue
		}
		v = strings.ReplaceAll(v, "${provider.api_key}", apiKey)
		v = strings.ReplaceAll(v, "${provider.api_base_url}", apiBaseURL)
		resolved[k] = v
	}
	return resolved
}
