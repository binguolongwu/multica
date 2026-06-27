package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// These tests verify the technical detail that an agent defined in the agent
// table can, at create/update time, pick up an LLM model configured in the
// llm_provider/llm_model config tables purely by setting agent.model to the
// model_code — the server auto-injects the matching provider's api_key and
// api_base_url into the agent's custom_env, and a later model change re-runs
// the injection. This is the server-side half of "dynamic model assignment";
// the daemon-side delivery of the model string to the CLI (--model flag / RPC)
// is already covered by existing daemon tests and the code in
// server/internal/daemon/daemon.go (two-tier resolution) + per-backend
// injection in server/pkg/agent/*.go.

// seedLLMProviderWithModel inserts an llm_provider + llm_model pair for the
// handler test workspace and registers cleanup. The modelCode is the string
// that agent.model will carry and that GetLLMProviderByModelCode matches on.
func seedLLMProviderWithModel(t *testing.T, code, modelCode, envVarKey, envVarBase, apiKey, apiBase string) {
	t.Helper()
	ctx := context.Background()
	queries := db.New(testPool)
	wsUUID := parseUUID(testWorkspaceID)

	provider, err := queries.CreateLLMProvider(ctx, db.CreateLLMProviderParams{
		WorkspaceID:   wsUUID,
		Name:          "Test Provider " + code,
		Code:          code,
		ApiType:       "openai",
		ApiBaseUrl:    apiBase,
		ApiKey:        apiKey,
		EnvVarApiKey:  envVarKey,
		EnvVarBaseUrl: envVarBase,
		Sort:          0,
	})
	if err != nil {
		t.Fatalf("seed llm_provider: %v", err)
	}

	if _, err := queries.CreateLLMModel(ctx, db.CreateLLMModelParams{
		WorkspaceID:  wsUUID,
		ProviderID:   provider.ID,
		Name:         "Test Model " + modelCode,
		ModelCode:    modelCode,
		Type:         1,
		Capabilities: []string{},
		Currency:     "CNY",
	}); err != nil {
		t.Fatalf("seed llm_model: %v", err)
	}

	t.Cleanup(func() {
		// Delete the model rows first (FK -> provider), then the provider.
		if _, err := testPool.Exec(ctx, `DELETE FROM llm_model WHERE provider_id = $1`, provider.ID); err != nil {
			t.Errorf("cleanup llm_model: %v", err)
		}
		if _, err := testPool.Exec(ctx, `DELETE FROM llm_provider WHERE id = $1`, provider.ID); err != nil {
			t.Errorf("cleanup llm_provider: %v", err)
		}
	})
}

// fetchAgentCustomEnv reads the persisted custom_env jsonb for an agent and
// returns it as a map. The HTTP response redacts env values (only
// has_custom_env / custom_env_key_count are exposed), so the actual injected
// values can only be asserted from the DB row.
func fetchAgentCustomEnv(t *testing.T, agentID string) map[string]string {
	t.Helper()
	var raw []byte
	if err := testPool.QueryRow(context.Background(), `SELECT custom_env FROM agent WHERE id = $1`, agentID).Scan(&raw); err != nil {
		t.Fatalf("load agent custom_env: %v", err)
	}
	env := map[string]string{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &env); err != nil {
			t.Fatalf("unmarshal custom_env %q: %v", string(raw), err)
		}
	}
	return env
}

func deleteTestAgent(t *testing.T, agentID string) {
	t.Helper()
	if agentID == "" {
		return
	}
	if _, err := testPool.Exec(context.Background(), `DELETE FROM agent WHERE id = $1`, agentID); err != nil {
		t.Errorf("cleanup agent: %v", err)
	}
}

// TestCreateAgent_AutoInjectsLLMCredentialsFromConfigTable verifies that
// creating an agent whose model matches an llm_model.model_code row causes the
// server to auto-inject the matching provider's api_key / api_base_url into the
// agent's custom_env (keyed by the provider's env_var_api_key / env_var_base_url).
func TestCreateAgent_AutoInjectsLLMCredentialsFromConfigTable(t *testing.T) {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	agentName := "LLM Inject Create " + suffix
	modelCode := "gpt-4o-" + suffix
	seedLLMProviderWithModel(t, "openai-"+suffix, modelCode, "OPENAI_API_KEY", "OPENAI_BASE_URL", "sk-test-secret", "https://api.example.com")

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/agents?workspace_id="+testWorkspaceID, map[string]any{
		"name":                 agentName,
		"runtime_id":           handlerTestRuntimeID(t),
		"model":                modelCode,
		"visibility":           "private",
		"max_concurrent_tasks": 1,
	})
	testHandler.CreateAgent(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateAgent: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp AgentResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	t.Cleanup(func() { deleteTestAgent(t, resp.ID) })

	if resp.Model != modelCode {
		t.Fatalf("response model = %q, want %q", resp.Model, modelCode)
	}

	env := fetchAgentCustomEnv(t, resp.ID)
	if env["OPENAI_API_KEY"] != "sk-test-secret" {
		t.Fatalf("custom_env OPENAI_API_KEY = %q, want sk-test-secret (full env: %v)", env["OPENAI_API_KEY"], env)
	}
	if env["OPENAI_BASE_URL"] != "https://api.example.com" {
		t.Fatalf("custom_env OPENAI_BASE_URL = %q, want https://api.example.com (full env: %v)", env["OPENAI_BASE_URL"], env)
	}
}

// TestUpdateAgent_DynamicallyChangesModelAndReinjectsCredentials is the core
// dynamic-switch test: an agent created with model A gets its model updated to
// model B, and the new provider's credentials are injected into custom_env
// while the previous provider's keys are preserved (inject never overwrites
// existing keys). This proves agent.model can be changed at runtime and the
// credentials follow.
func TestUpdateAgent_DynamicallyChangesModelAndReinjectsCredentials(t *testing.T) {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	agentName := "LLM Inject Update " + suffix
	modelA := "gpt-4o-" + suffix
	modelB := "claude-sonnet-4-" + suffix
	seedLLMProviderWithModel(t, "openai-"+suffix, modelA, "OPENAI_API_KEY", "OPENAI_BASE_URL", "sk-openai-secret", "https://api.openai.com")
	seedLLMProviderWithModel(t, "anthropic-"+suffix, modelB, "ANTHROPIC_API_KEY", "ANTHROPIC_BASE_URL", "sk-anthropic-secret", "https://api.anthropic.com")

	// Create the agent bound to model A.
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/agents?workspace_id="+testWorkspaceID, map[string]any{
		"name":                 agentName,
		"runtime_id":           handlerTestRuntimeID(t),
		"model":                modelA,
		"visibility":           "private",
		"max_concurrent_tasks": 1,
	})
	testHandler.CreateAgent(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateAgent: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var created AgentResponse
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode created response: %v", err)
	}
	t.Cleanup(func() { deleteTestAgent(t, created.ID) })

	env := fetchAgentCustomEnv(t, created.ID)
	if env["OPENAI_API_KEY"] != "sk-openai-secret" {
		t.Fatalf("after create: OPENAI_API_KEY = %q, want sk-openai-secret (env: %v)", env["OPENAI_API_KEY"], env)
	}

	// Dynamically switch the model A -> B via the update endpoint.
	w = httptest.NewRecorder()
	req = newRequest("PUT", "/api/agents/"+created.ID, map[string]any{
		"model": modelB,
	})
	req = withURLParam(req, "id", created.ID)
	testHandler.UpdateAgent(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateAgent: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var updated AgentResponse
	if err := json.NewDecoder(w.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated response: %v", err)
	}
	if updated.Model != modelB {
		t.Fatalf("response model = %q, want %q", updated.Model, modelB)
	}

	env = fetchAgentCustomEnv(t, created.ID)
	// The new provider's credentials must be injected...
	if env["ANTHROPIC_API_KEY"] != "sk-anthropic-secret" {
		t.Fatalf("after update: ANTHROPIC_API_KEY = %q, want sk-anthropic-secret (env: %v)", env["ANTHROPIC_API_KEY"], env)
	}
	if env["ANTHROPIC_BASE_URL"] != "https://api.anthropic.com" {
		t.Fatalf("after update: ANTHROPIC_BASE_URL = %q, want https://api.anthropic.com (env: %v)", env["ANTHROPIC_BASE_URL"], env)
	}
	// ...and the previous provider's key must be preserved (inject is additive).
	if env["OPENAI_API_KEY"] != "sk-openai-secret" {
		t.Fatalf("after update: OPENAI_API_KEY = %q, want preserved sk-openai-secret (inject must not overwrite)", env["OPENAI_API_KEY"])
	}
}

// TestCreateAgent_AutoInjectSkipsUnknownModelCode verifies the graceful no-op
// path: a model_code with no matching llm_model row must not error and must
// leave custom_env empty.
func TestCreateAgent_AutoInjectSkipsUnknownModelCode(t *testing.T) {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	agentName := "LLM Inject Unknown " + suffix

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/agents?workspace_id="+testWorkspaceID, map[string]any{
		"name":                 agentName,
		"runtime_id":           handlerTestRuntimeID(t),
		"model":                "no-such-model-" + suffix,
		"visibility":           "private",
		"max_concurrent_tasks": 1,
	})
	testHandler.CreateAgent(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateAgent: expected 201 for unknown model, got %d: %s", w.Code, w.Body.String())
	}
	var resp AgentResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	t.Cleanup(func() { deleteTestAgent(t, resp.ID) })

	env := fetchAgentCustomEnv(t, resp.ID)
	if len(env) != 0 {
		t.Fatalf("custom_env should be empty for unknown model_code, got %v", env)
	}
}

// TestCreateAgent_AutoInjectDoesNotOverwriteUserCustomEnv verifies that an
// explicit user-supplied custom_env key takes precedence over the provider's
// auto-injected value for the same env var — injection is additive only.
func TestCreateAgent_AutoInjectDoesNotOverwriteUserCustomEnv(t *testing.T) {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	agentName := "LLM Inject UserEnv " + suffix
	modelCode := "gpt-4o-userenv-" + suffix
	seedLLMProviderWithModel(t, "openai-"+suffix, modelCode, "OPENAI_API_KEY", "OPENAI_BASE_URL", "sk-provider-secret", "https://api.example.com")

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/agents?workspace_id="+testWorkspaceID, map[string]any{
		"name":       agentName,
		"runtime_id": handlerTestRuntimeID(t),
		"model":      modelCode,
		"custom_env": map[string]string{
			"OPENAI_API_KEY": "sk-user-set",
		},
		"visibility":           "private",
		"max_concurrent_tasks": 1,
	})
	testHandler.CreateAgent(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateAgent: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp AgentResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	t.Cleanup(func() { deleteTestAgent(t, resp.ID) })

	env := fetchAgentCustomEnv(t, resp.ID)
	// User-supplied key wins; the provider value must NOT overwrite it.
	if env["OPENAI_API_KEY"] != "sk-user-set" {
		t.Fatalf("custom_env OPENAI_API_KEY = %q, want user value sk-user-set (provider must not overwrite)", env["OPENAI_API_KEY"])
	}
	// The base_url was not user-supplied, so the provider value is injected.
	if env["OPENAI_BASE_URL"] != "https://api.example.com" {
		t.Fatalf("custom_env OPENAI_BASE_URL = %q, want provider value https://api.example.com (env: %v)", env["OPENAI_BASE_URL"], env)
	}
}

// TestResolveLLMEnvRefs_ResolvesProviderRefs verifies that ${provider.api_key}
// and ${provider.api_base_url} references in custom_env are resolved against
// the LLM provider matched by the agent's model_code at task-claim time.
func TestResolveLLMEnvRefs_ResolvesProviderRefs(t *testing.T) {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	modelCode := "gpt-4o-resolve-" + suffix
	seedLLMProviderWithModel(t, "openai-"+suffix, modelCode, "OPENAI_API_KEY", "OPENAI_BASE_URL", "sk-resolve-secret", "https://resolve.example.com")

	in := map[string]string{
		"OPENAI_API_KEY":  "${provider.api_key}",
		"OPENAI_BASE_URL": "${provider.api_base_url}",
	}
	got := testHandler.resolveLLMEnvRefs(context.Background(), in, testWorkspaceID, modelCode)
	if got["OPENAI_API_KEY"] != "sk-resolve-secret" {
		t.Fatalf("OPENAI_API_KEY = %q, want sk-resolve-secret (got: %v)", got["OPENAI_API_KEY"], got)
	}
	if got["OPENAI_BASE_URL"] != "https://resolve.example.com" {
		t.Fatalf("OPENAI_BASE_URL = %q, want https://resolve.example.com (got: %v)", got["OPENAI_BASE_URL"], got)
	}
}

// TestResolveLLMEnvRefs_ResolvesModelCode verifies ${model.code} resolves to
// the agent's model string without needing a provider lookup.
func TestResolveLLMEnvRefs_ResolvesModelCode(t *testing.T) {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	modelCode := "gpt-4o-modelref-" + suffix
	// Deliberately do NOT seed a provider: ${model.code} must resolve without one.
	in := map[string]string{
		"OPENAI_MODEL": "${model.code}",
	}
	got := testHandler.resolveLLMEnvRefs(context.Background(), in, testWorkspaceID, modelCode)
	if got["OPENAI_MODEL"] != modelCode {
		t.Fatalf("OPENAI_MODEL = %q, want %q", got["OPENAI_MODEL"], modelCode)
	}
}

// TestResolveLLMEnvRefs_LiteralsPassThrough verifies that custom_env values
// with no references are returned unchanged (fast path, no DB lookup).
func TestResolveLLMEnvRefs_LiteralsPassThrough(t *testing.T) {
	in := map[string]string{
		"FOO":     "bar",
		"API_KEY": "sk-literal",
	}
	got := testHandler.resolveLLMEnvRefs(context.Background(), in, testWorkspaceID, "any-model")
	if got["FOO"] != "bar" || got["API_KEY"] != "sk-literal" {
		t.Fatalf("literal values changed: got %v, want %v", got, in)
	}
	if len(got) != len(in) {
		t.Fatalf("map size changed: got %d, want %d", len(got), len(in))
	}
}

// TestResolveLLMEnvRefs_UnknownModelEmptyProviderRefs verifies that when the
// model matches no provider, ${provider.*} references resolve to empty while
// literals and ${model.code} are unaffected.
func TestResolveLLMEnvRefs_UnknownModelEmptyProviderRefs(t *testing.T) {
	in := map[string]string{
		"K":           "${provider.api_key}",
		"MODEL":       "${model.code}",
		"LITERAL":     "keep-me",
		"BASE_URL":    "${provider.api_base_url}",
	}
	got := testHandler.resolveLLMEnvRefs(context.Background(), in, testWorkspaceID, "no-such-model-"+fmt.Sprintf("%d", time.Now().UnixNano()))
	if got["K"] != "" {
		t.Fatalf("K = %q, want empty (no provider matched)", got["K"])
	}
	if got["BASE_URL"] != "" {
		t.Fatalf("BASE_URL = %q, want empty (no provider matched)", got["BASE_URL"])
	}
	if got["MODEL"] == "" {
		t.Fatalf("MODEL = empty, want the model code string (model.code resolves without a provider)")
	}
	if got["LITERAL"] != "keep-me" {
		t.Fatalf("LITERAL = %q, want keep-me", got["LITERAL"])
	}
}

// TestResolveLLMEnvRefs_EmbeddedRefs verifies references embedded inside a
// larger string are substituted in place (not just whole-value matches).
func TestResolveLLMEnvRefs_EmbeddedRefs(t *testing.T) {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	modelCode := "gpt-4o-embed-" + suffix
	seedLLMProviderWithModel(t, "openai-"+suffix, modelCode, "OPENAI_API_KEY", "OPENAI_BASE_URL", "sk-embed-secret", "https://embed.example.com")

	in := map[string]string{
		"COMPOSITE": "key=${provider.api_key} base=${provider.api_base_url} model=${model.code} tail",
	}
	got := testHandler.resolveLLMEnvRefs(context.Background(), in, testWorkspaceID, modelCode)
	want := "key=sk-embed-secret base=https://embed.example.com model=" + modelCode + " tail"
	if got["COMPOSITE"] != want {
		t.Fatalf("COMPOSITE = %q, want %q", got["COMPOSITE"], want)
	}
}

// TestLLMEnvVarsForRuntime pins the runtime→env-var registry that drives
// credential injection: each CLI reads a specific env var, and the registry is
// the authority (the provider's env_var_* is only a fallback for unmapped
// runtimes). Mapped runtimes with empty values (copilot/opencode) are
// intentional — they do not consume an LLM api_key env var (copilot uses a
// GitHub token; opencode is routed via OPENCODE_CONFIG_CONTENT).
func TestLLMEnvVarsForRuntime(t *testing.T) {
	t.Parallel()
	cases := []struct {
		provider    string
		wantAPIKey  string
		wantBaseURL string
		wantMapped  bool
	}{
		{"claude", "ANTHROPIC_API_KEY", "ANTHROPIC_BASE_URL", true},
		{"codex", "OPENAI_API_KEY", "OPENAI_BASE_URL", true},
		{"hermes", "GLM_API_KEY", "", true},
		{"copilot", "", "", true},  // GitHub token, not an LLM api_key
		{"opencode", "", "", true}, // routed via OPENCODE_CONFIG_CONTENT provider injection
		{"handler_test_runtime", "", "", false}, // unmapped → caller falls back to provider.env_var_*
		{"openclaw", "", "", false},             // unmapped
	}
	for _, tc := range cases {
		ak, bu, mapped := llmEnvVarsForRuntime(tc.provider)
		if ak != tc.wantAPIKey || bu != tc.wantBaseURL || mapped != tc.wantMapped {
			t.Fatalf("provider %q: got (key=%q base=%q mapped=%v), want (key=%q base=%q mapped=%v)",
				tc.provider, ak, bu, mapped, tc.wantAPIKey, tc.wantBaseURL, tc.wantMapped)
		}
	}
}
