# Zeroclaw Runtime Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add "zeroclaw" as a new agent backend type in multica, supporting local mode (ACP JSON-RPC stdio) and gateway mode (WebSocket).

**Architecture:** Single new file `server/pkg/agent/zeroclaw.go` containing `zeroclawBackend` (implements `agent.Backend`), `zeroclawClient` (self-contained JSON-RPC 2.0 stdio transport for local mode), and `zeroclawGatewayClient` (self-contained WebSocket transport for gateway mode). No shared state with other backends. Mode selected via `ExecOptions.ZeroclawMode`.

**Tech Stack:** Go stdlib (os/exec, encoding/json, bufio), gorilla/websocket (already in project via daemonws), TypeScript type union.

## Global Constraints

- Zero coupling to other backend files (hermes.go, openclaw.go, etc.) — zeroclawClient and zeroclawGatewayClient are fully self-contained.
- Follow existing patterns: `ZeroclawMode` mirrors `OpenclawMode`; blocked args pattern mirrors `hermesBlockedArgs`/`openclawBlockedArgs`.
- Gateway mode WebSocket uses `gorilla/websocket` (already a project dependency via `daemonws`).
- No database migrations — existing `agent_runtime.provider` column already accepts arbitrary strings.
- No changes to `server/internal/handler/` — provider flows from daemon registration `type` field.

---

## File Structure

- **Create:** `server/pkg/agent/zeroclaw.go` — zeroclawBackend + zeroclawClient (ACP stdio) + zeroclawGatewayClient (WebSocket)
- **Modify:** `server/pkg/agent/agent.go` — add `"zeroclaw"` to SupportedTypes, New() switch, launchHeaders, ExecOptions.ZeroclawMode
- **Modify:** `packages/core/types/agent.ts` — add `"zeroclaw"` to RUNTIME_PROFILE_PROTOCOL_FAMILIES
- **Create:** `server/pkg/agent/zeroclaw_test.go` — unit tests for both modes

---

### Task 1: Register zeroclaw type + skeleton

**Files:**
- Create: `server/pkg/agent/zeroclaw.go`
- Modify: `server/pkg/agent/agent.go`
- Modify: `packages/core/types/agent.ts`

**Interfaces:**
- Consumes: nothing (first task)
- Produces: `zeroclawBackend` stub (compiles, Execute returns "not implemented"); `zeroclaw` registered in SupportedTypes, New(), launchHeaders; ExecOptions.ZeroclawMode field; TypeScript type union extended

- [ ] **Step 1: Create zeroclaw.go skeleton**

Create `server/pkg/agent/zeroclaw.go`:

```go
package agent

import (
	"context"
	"fmt"
)

// zeroclawBlockedArgs are flags hardcoded by the daemon that must not be
// overridden by user-configured custom_args.
var zeroclawBlockedArgs = map[string]blockedArgMode{
	"acp":     blockedStandalone,  // local mode must use acp protocol
	"-a":      blockedWithValue,   // agent alias managed by multica
	"--alias": blockedWithValue,
}

// zeroclawBackend implements Backend by spawning `zeroclaw acp` (local mode)
// or connecting to a zeroclaw gateway via WebSocket (gateway mode).
type zeroclawBackend struct {
	cfg Config
}

func (b *zeroclawBackend) Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	return nil, fmt.Errorf("zeroclaw: not yet implemented")
}
```

- [ ] **Step 2: Register zeroclaw in agent.go — SupportedTypes**

In `server/pkg/agent/agent.go`, add `"zeroclaw"` to the `SupportedTypes` slice (line ~156, before the closing `]`):

```go
var SupportedTypes = []string{
	"claude",
	"codebuddy",
	"codex",
	"copilot",
	"opencode",
	"openclaw",
	"hermes",
	"pi",
	"cursor",
	"kimi",
	"kiro",
	"antigravity",
	"zeroclaw",   // <-- add this line
}
```

- [ ] **Step 3: Register zeroclaw in agent.go — New() switch**

In `server/pkg/agent/agent.go`, add a case to the `New()` switch (after the `"antigravity"` case, before `"qoder"`):

```go
case "zeroclaw":
	return &zeroclawBackend{cfg: cfg}, nil
```

- [ ] **Step 4: Register zeroclaw in agent.go — launchHeaders**

In `server/pkg/agent/agent.go`, add to the `launchHeaders` map:

```go
"zeroclaw": "zeroclaw acp",
```

- [ ] **Step 5: Add ZeroclawMode to ExecOptions**

In `server/pkg/agent/agent.go`, add to the `ExecOptions` struct (after `OpenclawMode`, around line 59):

```go
// ZeroclawMode chooses between local (ACP stdio) and gateway (WebSocket)
// routing for the zeroclaw backend. "" or "local" keeps the default
// behaviour — the daemon spawns `zeroclaw acp` and the agent loop runs
// in-process on the daemon host. "gateway" connects to a remote zeroclaw
// gateway via WebSocket. Other backends ignore this field.
ZeroclawMode string
```

- [ ] **Step 6: Add zeroclaw to TypeScript type union**

In `packages/core/types/agent.ts`, add `"zeroclaw"` to the `RUNTIME_PROFILE_PROTOCOL_FAMILIES` array (line ~71):

```typescript
export const RUNTIME_PROFILE_PROTOCOL_FAMILIES = [
  "claude",
  "codebuddy",
  "codex",
  "copilot",
  "opencode",
  "openclaw",
  "hermes",
  "pi",
  "cursor",
  "kimi",
  "kiro",
  "antigravity",
  "zeroclaw",   // <-- add this line
] as const;
```

- [ ] **Step 7: Verify compilation**

```bash
cd /home/longwu/multica && go build ./server/pkg/agent/
```

Expected: compiles successfully (Execute returns "not yet implemented" at runtime, which is fine).

- [ ] **Step 8: Commit**

```bash
git add server/pkg/agent/zeroclaw.go server/pkg/agent/agent.go packages/core/types/agent.ts
git commit -m "feat(agent): register zeroclaw runtime type skeleton"
```

---

### Task 2: Implement local mode (ACP stdio)

**Files:**
- Modify: `server/pkg/agent/zeroclaw.go`
- Create: `server/pkg/agent/zeroclaw_test.go`

**Interfaces:**
- Consumes: `agent.Backend`, `agent.ExecOptions`, `agent.Config`, `agent.Session`, `agent.Message`, `agent.Result`, `agent.blockedArgMode`, `agent.blockedStandalone`, `agent.blockedWithValue`, `agent.filterCustomArgs`, `agent.buildEnv`, `agent.hideAgentWindow`, `agent.newLogWriter`, `agent.trySend`, `agent.runContext` — all from the `agent` package
- Produces: `zeroclawClient` (JSON-RPC 2.0 stdio transport), `executeLocal()` method on zeroclawBackend, `buildZeroclawArgs()` helper

- [ ] **Step 1: Define JSON-RPC types in zeroclaw.go**

Add these types above the `zeroclawBackend` struct in `server/pkg/agent/zeroclaw.go`:

```go
import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ── JSON-RPC 2.0 types ──────────────────────────────────────────

type zcRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      int             `json:"id"`
}

type zcRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type zcRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *zcRPCError     `json:"error,omitempty"`
	ID      int             `json:"id"`
}

type zcRPCNotification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type zcPendingCall struct {
	response chan zcRPCResponse
}

// ── ACP notification shapes ──────────────────────────────────────

// zcACPUpdate is the `params.update` object inside a session/update notification.
type zcACPUpdate struct {
	SessionUpdate string          `json:"sessionUpdate"`
	ToolCallID    string          `json:"toolCallId,omitempty"`
	Name          string          `json:"name,omitempty"`
	Title         string          `json:"title,omitempty"`
	Kind          string          `json:"kind,omitempty"`
	Status        string          `json:"status,omitempty"`
	RawInput      json.RawMessage `json:"rawInput,omitempty"`
	RawOutput     string          `json:"rawOutput,omitempty"`
	Body          string          `json:"body,omitempty"`
	Content       json.RawMessage `json:"content,omitempty"`
}

type zcACPSessionUpdateParams struct {
	SessionID string       `json:"sessionId"`
	Update    zcACPUpdate  `json:"update"`
}

// zcACPPromptResult is the success response from session/prompt.
type zcACPPromptResult struct {
	StopReason string `json:"stopReason"`
}
```

- [ ] **Step 2: Define zeroclawClient struct and constructor**

Add after the JSON-RPC types in `server/pkg/agent/zeroclaw.go`:

```go
// zeroclawClient manages JSON-RPC 2.0 communication over stdin/stdout
// with a `zeroclaw acp` subprocess. Fully self-contained — does not
// share any types or state with hermesClient.
type zeroclawClient struct {
	stdin   io.WriteCloser
	writeMu sync.Mutex
	mu      sync.Mutex
	nextID  int
	pending map[int]*zcPendingCall

	// Callbacks set by the caller.
	onMessage func(Message)
}

func newZeroclawClient(stdin io.WriteCloser) *zeroclawClient {
	return &zeroclawClient{
		stdin:   stdin,
		pending: make(map[int]*zcPendingCall),
	}
}

// request sends a JSON-RPC request and waits for the response.
func (c *zeroclawClient) request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	id := c.nextID
	c.nextID++
	ch := make(chan zcRPCResponse, 1)
	c.pending[id] = &zcPendingCall{response: ch}
	c.mu.Unlock()

	rawParams, _ := json.Marshal(params)
	req := zcRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  rawParams,
		ID:      id,
	}
	if err := c.writeJSON(req); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("zeroclaw: write %s: %w", method, err)
	}

	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("zeroclaw: %s failed (code %d): %s", method, resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	}
}

// writeJSON serialises v as a single JSON line to stdin.
func (c *zeroclawClient) writeJSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err = fmt.Fprintf(c.stdin, "%s\n", data)
	return err
}

// handleLine dispatches a single line from stdout: either a response
// matching a pending request, or a notification (session/update).
func (c *zeroclawClient) handleLine(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	// Try as a response first (has "id" field).
	var resp zcRPCResponse
	if err := json.Unmarshal([]byte(line), &resp); err == nil && resp.JSONRPC == "2.0" && resp.ID != 0 {
		c.mu.Lock()
		pc, ok := c.pending[resp.ID]
		if ok {
			delete(c.pending, resp.ID)
		}
		c.mu.Unlock()
		if ok {
			select {
			case pc.response <- resp:
			default:
			}
		}
		return
	}

	// Try as a notification (no "id" field).
	var notif zcRPCNotification
	if err := json.Unmarshal([]byte(line), &notif); err != nil || notif.Method != "session/update" {
		return
	}

	var updateParams zcACPSessionUpdateParams
	if err := json.Unmarshal(notif.Params, &updateParams); err != nil {
		return
	}
	c.handleSessionUpdate(updateParams.Update)
}

// handleSessionUpdate converts an ACP session/update notification into
// a multica Message and sends it to onMessage.
func (c *zeroclawClient) handleSessionUpdate(upd zcACPUpdate) {
	if c.onMessage == nil {
		return
	}
	switch upd.SessionUpdate {
	case "agent_message_chunk":
		c.extractTextContent(upd.Content, MessageText)
	case "agent_thought_chunk":
		c.extractTextContent(upd.Content, MessageThinking)
	case "tool_call":
		var args map[string]any
		if len(upd.RawInput) > 0 {
			json.Unmarshal(upd.RawInput, &args)
		}
		c.onMessage(Message{
			Type:   MessageToolUse,
			Tool:   upd.Name,
			CallID: upd.ToolCallID,
			Input:  args,
		})
	case "tool_call_update":
		output := upd.Body
		if output == "" {
			output = upd.RawOutput
		}
		c.onMessage(Message{
			Type:   MessageToolResult,
			Tool:   upd.Name,
			CallID: upd.ToolCallID,
			Output: output,
		})
	}
}

// extractTextContent extracts text from ACP content blocks and sends
// a Message of the given type. Handles both single text objects and
// arrays of content blocks.
func (c *zeroclawClient) extractTextContent(raw json.RawMessage, msgType MessageType) {
	if len(raw) == 0 {
		return
	}
	// Try as {"type":"text","text":"..."} (single object)
	var single struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &single); err == nil && single.Type == "text" && single.Text != "" {
		c.onMessage(Message{Type: msgType, Content: single.Text})
		return
	}
	// Try as [{"type":"content","content":{"type":"text","text":"..."}}] (array)
	var arr []struct {
		Type    string `json:"type"`
		Content struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &arr); err == nil {
		for _, item := range arr {
			if item.Content.Text != "" {
				c.onMessage(Message{Type: msgType, Content: item.Content.Text})
			}
		}
	}
}

// closeAllPending resolves every outstanding request with err.
func (c *zeroclawClient) closeAllPending(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, pc := range c.pending {
		select {
		case pc.response <- zcRPCResponse{
			Error: &zcRPCError{Code: -1, Message: err.Error()},
		}:
		default:
		}
		delete(c.pending, id)
	}
}
```

- [ ] **Step 3: Implement buildZeroclawArgs and executeLocal**

Replace the stub `Execute` and add local mode implementation in `server/pkg/agent/zeroclaw.go`:

```go
// buildZeroclawArgs assembles the argv for a `zeroclaw acp` invocation.
// Always starts with "acp"; appends filtered custom_args.
func buildZeroclawArgs(opts ExecOptions, logger *slog.Logger) []string {
	args := []string{"acp"}
	customArgs := filterCustomArgs(opts.CustomArgs, zeroclawBlockedArgs, logger)
	args = append(args, customArgs...)
	return args
}

func (b *zeroclawBackend) Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	if opts.ZeroclawMode == "gateway" {
		return b.executeGateway(ctx, prompt, opts)
	}
	return b.executeLocal(ctx, prompt, opts)
}

func (b *zeroclawBackend) executeLocal(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	execPath := b.cfg.ExecutablePath
	if execPath == "" {
		execPath = "zeroclaw"
	}
	if _, err := exec.LookPath(execPath); err != nil {
		return nil, fmt.Errorf("zeroclaw executable not found at %q: %w", execPath, err)
	}

	timeout := opts.Timeout
	runCtx, cancel := runContext(ctx, timeout)

	args := buildZeroclawArgs(opts, b.cfg.Logger)
	cmd := exec.CommandContext(runCtx, execPath, args...)
	hideAgentWindow(cmd)
	b.cfg.Logger.Info("agent command", "exec", execPath, "args", args)
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	}
	cmd.Env = buildEnv(b.cfg.Env)
	// Zeroclaw ACP needs YOLO mode so tools auto-approve.
	cmd.Env = append(cmd.Env, "ZEROCLAW_UNSUPERVISED=1")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("zeroclaw stdout pipe: %w", err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("zeroclaw stdin pipe: %w", err)
	}
	cmd.Stderr = newLogWriter(b.cfg.Logger, "[zeroclaw:stderr] ")

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start zeroclaw: %w", err)
	}

	b.cfg.Logger.Info("zeroclaw acp started", "pid", cmd.Process.Pid, "cwd", opts.Cwd)

	msgCh := make(chan Message, 256)
	resCh := make(chan Result, 1)

	var outputMu sync.Mutex
	var output strings.Builder
	var streamingCurrentTurn atomic.Bool

	client := newZeroclawClient(stdin)
	client.onMessage = func(msg Message) {
		if !streamingCurrentTurn.Load() {
			return
		}
		if msg.Type == MessageText {
			outputMu.Lock()
			output.WriteString(msg.Content)
			outputMu.Unlock()
		}
		trySend(msgCh, msg)
	}

	// Read stdout in background.
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
		for scanner.Scan() {
			client.handleLine(scanner.Text())
		}
		client.closeAllPending(fmt.Errorf("zeroclaw process exited"))
	}()

	// Drive the ACP session lifecycle.
	go func() {
		defer cancel()
		defer close(msgCh)
		defer close(resCh)
		defer func() {
			stdin.Close()
			_ = cmd.Wait()
		}()

		startTime := time.Now()
		finalStatus := "completed"
		var finalError string
		var sessionID string
		effectiveModel := strings.TrimSpace(opts.Model)

		// 1. Initialize handshake.
		initResult, err := client.request(runCtx, "initialize", map[string]any{
			"protocolVersion": 1,
			"clientInfo": map[string]any{
				"name":    "multica-agent-sdk",
				"version": "0.2.0",
			},
		})
		if err != nil {
			finalStatus = "failed"
			finalError = fmt.Sprintf("zeroclaw initialize failed: %v", err)
			resCh <- Result{Status: finalStatus, Error: finalError, DurationMs: time.Since(startTime).Milliseconds()}
			return
		}

		// 2. Create or resume session.
		cwd := opts.Cwd
		if cwd == "" {
			cwd = "."
		}
		agentAlias := effectiveModel
		if agentAlias == "" {
			agentAlias = "default"
		}

		if opts.ResumeSessionID != "" {
			result, err := client.request(runCtx, "session/resume", map[string]any{
				"cwd":       cwd,
				"sessionId": opts.ResumeSessionID,
			})
			if err != nil {
				finalStatus = "failed"
				finalError = fmt.Sprintf("zeroclaw session/resume failed: %v", err)
				resCh <- Result{Status: finalStatus, Error: finalError, DurationMs: time.Since(startTime).Milliseconds()}
				return
			}
			sessionID = extractZCSessionID(result)
			if sessionID == "" {
				sessionID = opts.ResumeSessionID
			}
		} else {
			result, err := client.request(runCtx, "session/new", map[string]any{
				"cwd":        cwd,
				"agentAlias": agentAlias,
			})
			if err != nil {
				finalStatus = "failed"
				finalError = fmt.Sprintf("zeroclaw session/new failed: %v", err)
				resCh <- Result{Status: finalStatus, Error: finalError, DurationMs: time.Since(startTime).Milliseconds()}
				return
			}
			sessionID = extractZCSessionID(result)
			if sessionID == "" {
				finalStatus = "failed"
				finalError = "zeroclaw session/new returned no session ID"
				resCh <- Result{Status: finalStatus, Error: finalError, DurationMs: time.Since(startTime).Milliseconds()}
				return
			}
		}

		b.cfg.Logger.Info("zeroclaw session created", "session_id", sessionID)

		// 3. Send the prompt.
		streamingCurrentTurn.Store(true)
		_, err = client.request(runCtx, "session/prompt", map[string]any{
			"sessionId": sessionID,
			"prompt": []map[string]any{
				{"type": "text", "text": prompt},
			},
		})
		if err != nil {
			if runCtx.Err() == context.DeadlineExceeded {
				finalStatus = "timeout"
				finalError = fmt.Sprintf("zeroclaw timed out after %s", timeout)
			} else if runCtx.Err() == context.Canceled {
				finalStatus = "aborted"
				finalError = "execution cancelled"
			} else {
				finalStatus = "failed"
				finalError = fmt.Sprintf("zeroclaw session/prompt failed: %v", err)
			}
		}

		duration := time.Since(startTime)
		b.cfg.Logger.Info("zeroclaw finished", "pid", cmd.Process.Pid, "status", finalStatus, "duration", duration.Round(time.Millisecond).String())

		stdin.Close()
		cancel()
		<-readerDone

		outputMu.Lock()
		finalOutput := output.String()
		outputMu.Unlock()

		resCh <- Result{
			Status:     finalStatus,
			Output:     finalOutput,
			Error:      finalError,
			DurationMs: duration.Milliseconds(),
			SessionID:  sessionID,
		}
	}()

	return &Session{Messages: msgCh, Result: resCh}, nil
}

// extractZCSessionID extracts the sessionId from a JSON-RPC result.
func extractZCSessionID(result json.RawMessage) string {
	var v struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(result, &v); err != nil {
		return ""
	}
	return v.SessionID
}
```

- [ ] **Step 4: Write local mode unit tests**

Create `server/pkg/agent/zeroclaw_test.go`:

```go
package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildZeroclawArgs(t *testing.T) {
	logger := testLogger()

	t.Run("default local mode", func(t *testing.T) {
		args := buildZeroclawArgs(ExecOptions{}, logger)
		if len(args) == 0 || args[0] != "acp" {
			t.Fatalf("expected args to start with 'acp', got %v", args)
		}
	})

	t.Run("custom args not blocked", func(t *testing.T) {
		args := buildZeroclawArgs(ExecOptions{
			CustomArgs: []string{"--verbose", "--log-level", "debug"},
		}, logger)
		found := false
		for _, a := range args {
			if a == "--verbose" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected --verbose in args, got %v", args)
		}
	})

	t.Run("blocked args are filtered", func(t *testing.T) {
		args := buildZeroclawArgs(ExecOptions{
			CustomArgs: []string{"-a", "my-agent", "--help"},
		}, logger)
		for _, a := range args {
			if a == "-a" || a == "--alias" {
				t.Errorf("blocked arg %q should have been filtered, got %v", a, args)
			}
		}
	})
}

func TestZeroclawClientHandleLine(t *testing.T) {
	stdin := &nopWriteCloser{}
	client := newZeroclawClient(stdin)

	t.Run("parses agent_message_chunk", func(t *testing.T) {
		var gotMsg Message
		client.onMessage = func(m Message) { gotMsg = m }

		notif := `{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"s1","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"Hello"}}}}`
		client.handleLine(notif)

		if gotMsg.Type != MessageText || gotMsg.Content != "Hello" {
			t.Errorf("expected text message 'Hello', got type=%q content=%q", gotMsg.Type, gotMsg.Content)
		}
	})

	t.Run("parses tool_call", func(t *testing.T) {
		var gotMsg Message
		client.onMessage = func(m Message) { gotMsg = m }

		notif := `{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"s1","update":{"sessionUpdate":"tool_call","toolCallId":"call_1","name":"shell","title":"shell","kind":"bash","status":"pending","rawInput":{"command":"pwd"}}}}`
		client.handleLine(notif)

		if gotMsg.Type != MessageToolUse || gotMsg.Tool != "shell" || gotMsg.CallID != "call_1" {
			t.Errorf("expected tool_use, got type=%q tool=%q callID=%q", gotMsg.Type, gotMsg.Tool, gotMsg.CallID)
		}
	})

	t.Run("parses tool_call_update", func(t *testing.T) {
		var gotMsg Message
		client.onMessage = func(m Message) { gotMsg = m }

		notif := `{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"s1","update":{"sessionUpdate":"tool_call_update","toolCallId":"call_1","name":"shell","title":"shell","kind":"bash","status":"completed","rawOutput":"/home/user\n","body":"/home/user\n"}}`
		client.handleLine(notif)

		if gotMsg.Type != MessageToolResult || gotMsg.Tool != "shell" || gotMsg.Output != "/home/user\n" {
			t.Errorf("expected tool_result, got type=%q tool=%q output=%q", gotMsg.Type, gotMsg.Tool, gotMsg.Output)
		}
	})

	t.Run("parses agent_thought_chunk", func(t *testing.T) {
		var gotMsg Message
		client.onMessage = func(m Message) { gotMsg = m }

		notif := `{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"s1","update":{"sessionUpdate":"agent_thought_chunk","content":{"type":"text","text":"Let me think..."}}}`
		client.handleLine(notif)

		if gotMsg.Type != MessageThinking || gotMsg.Content != "Let me think..." {
			t.Errorf("expected thinking message, got type=%q content=%q", gotMsg.Type, gotMsg.Content)
		}
	})

	t.Run("ignores unknown notification methods", func(t *testing.T) {
		var called bool
		client.onMessage = func(m Message) { called = true }

		notif := `{"jsonrpc":"2.0","method":"something/else","params":{}}`
		client.handleLine(notif)

		if called {
			t.Error("expected onMessage not to be called for unknown notification")
		}
	})
}

func TestZeroclawClientRequestResponse(t *testing.T) {
	stdin := &nopWriteCloser{}
	client := newZeroclawClient(stdin)

	// Simulate server pushing a response for request id=0.
	resp := zcRPCResponse{
		JSONRPC: "2.0",
		Result:  json.RawMessage(`{"sessionId":"abc123"}`),
		ID:      0,
	}
	data, _ := json.Marshal(resp)

	// Pre-load the pending map before calling handleLine.
	// (In real use, request() would have set this up.)
	client.mu.Lock()
	ch := make(chan zcRPCResponse, 1)
	client.pending[0] = &zcPendingCall{response: ch}
	client.mu.Unlock()

	go func() {
		client.handleLine(string(data))
	}()

	raw, err := client.request(t.Context(), "session/new", map[string]any{
		"cwd":        "/tmp",
		"agentAlias": "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var v struct {
		SessionID string `json:"sessionId"`
	}
	json.Unmarshal(raw, &v)
	if v.SessionID != "abc123" {
		t.Errorf("expected sessionId=abc123, got %q", v.SessionID)
	}
}

func TestExtractZCSessionID(t *testing.T) {
	result := json.RawMessage(`{"sessionId":"ses_abc123","other":"field"}`)
	id := extractZCSessionID(result)
	if id != "ses_abc123" {
		t.Errorf("expected ses_abc123, got %q", id)
	}

	bad := json.RawMessage(`{"noSessionId":true}`)
	if id := extractZCSessionID(bad); id != "" {
		t.Errorf("expected empty, got %q", id)
	}
}

func TestZeroclawBackendNew(t *testing.T) {
	backend, err := New("zeroclaw", Config{})
	if err != nil {
		t.Fatalf("New(zeroclaw): %v", err)
	}
	if backend == nil {
		t.Fatal("expected non-nil backend")
	}
	_, ok := backend.(*zeroclawBackend)
	if !ok {
		t.Fatalf("expected *zeroclawBackend, got %T", backend)
	}
}

func TestZeroclawIsSupportedType(t *testing.T) {
	if !IsSupportedType("zeroclaw") {
		t.Error("expected zeroclaw to be a supported type")
	}
}

// ── helpers ──

func testLogger() *slog.Logger {
	return slog.Default()
}

type nopWriteCloser struct{}

func (n *nopWriteCloser) Write(p []byte) (int, error) { return len(p), nil }
func (n *nopWriteCloser) Close() error                 { return nil }
```

- [ ] **Step 5: Run unit tests**

```bash
cd /home/longwu/multica && go test ./server/pkg/agent/ -run "TestZeroclaw|TestBuildZeroclaw" -v -count=1
```

Expected: all tests pass.

- [ ] **Step 6: Verify compilation of full project**

```bash
cd /home/longwu/multica && go build ./server/...
```

Expected: compiles successfully.

- [ ] **Step 7: Commit**

```bash
git add server/pkg/agent/zeroclaw.go server/pkg/agent/zeroclaw_test.go
git commit -m "feat(agent): implement zeroclaw local mode via ACP stdio"
```

---

### Task 3: Implement gateway mode (WebSocket)

**Files:**
- Modify: `server/pkg/agent/zeroclaw.go`
- Modify: `server/pkg/agent/zeroclaw_test.go`

**Interfaces:**
- Consumes: `gorilla/websocket` (already in go.mod via daemonws)
- Produces: `zeroclawGatewayClient` (WebSocket transport), `executeGateway()` method

- [ ] **Step 1: Add gorilla/websocket import and gateway types**

Add to the import block at the top of `server/pkg/agent/zeroclaw.go`:

```go
	import (
		// ... existing imports ...
		"net/http"

		"github.com/gorilla/websocket"
	)
```

Add these gateway types after the ACP notification types:

```go
// ── Gateway WebSocket message types ──────────────────────────────

type zcGWMessage struct {
	Type         string          `json:"type"`
	Content      string          `json:"content,omitempty"`
	Name         string          `json:"name,omitempty"`
	Args         json.RawMessage `json:"args,omitempty"`
	CallID       string          `json:"call_id,omitempty"`
	Output       string          `json:"output,omitempty"`
	FullResponse string          `json:"full_response,omitempty"`
	SessionID    string          `json:"session_id,omitempty"`
	Resumed      bool            `json:"resumed,omitempty"`
	Error        string          `json:"error,omitempty"`
	// Approval fields
	RequestID        string `json:"request_id,omitempty"`
	Tool             string `json:"tool,omitempty"`
	ArgumentsSummary string `json:"arguments_summary,omitempty"`
	TimeoutSecs      int    `json:"timeout_secs,omitempty"`
	Decision         string `json:"decision,omitempty"`
}
```

- [ ] **Step 2: Implement executeGateway**

Add the `executeGateway` method to `zeroclawBackend` in `server/pkg/agent/zeroclaw.go`:

```go
func (b *zeroclawBackend) executeGateway(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	gatewayURL := b.resolveGatewayURL(opts)
	if gatewayURL == "" {
		return nil, fmt.Errorf("zeroclaw gateway URL not configured — set ZEROCLAW_GATEWAY_URL in agent custom_env")
	}

	msgCh := make(chan Message, 256)
	resCh := make(chan Result, 1)

	go func() {
		defer close(msgCh)
		defer close(resCh)

		startTime := time.Now()
		finalStatus := "completed"
		var finalError string
		var output strings.Builder
		var sessionID string

		// Connect to gateway.
		conn, _, err := websocket.DefaultDialer.DialContext(ctx, gatewayURL, b.gatewayHeaders(opts))
		if err != nil {
			resCh <- Result{
				Status:     "failed",
				Error:      fmt.Sprintf("zeroclaw gateway unreachable: %v", err),
				DurationMs: time.Since(startTime).Milliseconds(),
			}
			return
		}
		defer conn.Close()

		// Send the prompt.
		sendMsg := zcGWMessage{
			Type:    "message",
			Content: prompt,
		}
		if err := conn.WriteJSON(sendMsg); err != nil {
			resCh <- Result{
				Status:     "failed",
				Error:      fmt.Sprintf("zeroclaw gateway write failed: %v", err),
				DurationMs: time.Since(startTime).Milliseconds(),
			}
			return
		}

		// Read loop.
		for {
			var msg zcGWMessage
			if err := conn.ReadJSON(&msg); err != nil {
				if finalStatus == "completed" {
					finalStatus = "failed"
					finalError = fmt.Sprintf("zeroclaw gateway connection lost: %v", err)
				}
				break
			}

			switch msg.Type {
			case "session_start":
				sessionID = msg.SessionID
				b.cfg.Logger.Info("zeroclaw gateway session started", "session_id", sessionID)
			case "chunk":
				output.WriteString(msg.Content)
				trySend(msgCh, Message{Type: MessageText, Content: msg.Content})
			case "tool_call":
				var args map[string]any
				if len(msg.Args) > 0 {
					json.Unmarshal(msg.Args, &args)
				}
				trySend(msgCh, Message{
					Type:   MessageToolUse,
					Tool:   msg.Name,
					CallID: msg.CallID,
					Input:  args,
				})
			case "tool_result":
				trySend(msgCh, Message{
					Type:   MessageToolResult,
					Tool:   msg.Name,
					CallID: msg.CallID,
					Output: msg.Output,
				})
			case "thinking":
				trySend(msgCh, Message{Type: MessageThinking, Content: msg.Content})
			case "done":
				// Normal completion — exit the read loop.
				resCh <- Result{
					Status:     finalStatus,
					Output:     output.String(),
					DurationMs: time.Since(startTime).Milliseconds(),
					SessionID:  sessionID,
				}
				return
			case "error":
				finalStatus = "failed"
				finalError = msg.Error
				if finalError == "" {
					finalError = msg.Content
				}
				trySend(msgCh, Message{Type: MessageError, Content: finalError})
			case "approval_request":
				// Auto-approve in daemon context.
				_ = conn.WriteJSON(zcGWMessage{
					Type:      "approval_response",
					RequestID: msg.RequestID,
					Decision:  "always",
				})
			}
		}

		// If we exit the loop without a "done" message, send the result.
		select {
		case resCh <- Result{
			Status:     finalStatus,
			Output:     output.String(),
			Error:      finalError,
			DurationMs: time.Since(startTime).Milliseconds(),
			SessionID:  sessionID,
		}:
		default:
		}
	}()

	return &Session{Messages: msgCh, Result: resCh}, nil
}

// resolveGatewayURL reads the gateway WebSocket URL from agent custom_env.
// Looks for ZEROCLAW_GATEWAY_URL in the backend's configured environment.
func (b *zeroclawBackend) resolveGatewayURL(opts ExecOptions) string {
	for _, e := range b.cfg.Env {
		if after, ok := strings.CutPrefix(e, "ZEROCLAW_GATEWAY_URL="); ok {
			return after
		}
	}
	return ""
}

// gatewayHeaders builds HTTP headers for the gateway WebSocket handshake,
// including an optional bearer token from custom_env.
func (b *zeroclawBackend) gatewayHeaders(opts ExecOptions) http.Header {
	h := http.Header{}
	for _, e := range b.cfg.Env {
		if after, ok := strings.CutPrefix(e, "ZEROCLAW_GATEWAY_TOKEN="); ok {
			h.Set("Authorization", "Bearer "+after)
			break
		}
	}
	return h
}
```

- [ ] **Step 3: Add gateway mode unit tests**

Append to `server/pkg/agent/zeroclaw_test.go`:

```go
func TestZeroclawResolveGatewayURL(t *testing.T) {
	b := &zeroclawBackend{cfg: Config{
		Env: map[string]string{
			"ZEROCLAW_GATEWAY_URL":   "ws://192.168.1.100:42617",
			"ZEROCLAW_GATEWAY_TOKEN": "test-token-123",
		},
	}}

	url := b.resolveGatewayURL(ExecOptions{})
	if url != "ws://192.168.1.100:42617" {
		t.Errorf("expected gateway URL, got %q", url)
	}

	headers := b.gatewayHeaders(ExecOptions{})
	if got := headers.Get("Authorization"); got != "Bearer test-token-123" {
		t.Errorf("expected Bearer token, got %q", got)
	}
}

func TestZeroclawResolveGatewayURLMissing(t *testing.T) {
	b := &zeroclawBackend{cfg: Config{}}
	url := b.resolveGatewayURL(ExecOptions{})
	if url != "" {
		t.Errorf("expected empty URL when not configured, got %q", url)
	}
}

func TestZeroclawExecuteGatewayNoURL(t *testing.T) {
	b := &zeroclawBackend{cfg: Config{}}
	_, err := b.Execute(t.Context(), "test prompt", ExecOptions{ZeroclawMode: "gateway"})
	if err == nil {
		t.Fatal("expected error when gateway URL is not configured")
	}
	if !strings.Contains(err.Error(), "gateway URL not configured") {
		t.Errorf("expected 'gateway URL not configured' error, got: %v", err)
	}
}

func TestZeroclawExecuteModeDispatch(t *testing.T) {
	b := &zeroclawBackend{cfg: Config{
		ExecutablePath: "/nonexistent/zeroclaw",
	}}

	// gateway mode with no URL should fail fast with config error.
	_, err := b.Execute(t.Context(), "hello", ExecOptions{ZeroclawMode: "gateway"})
	if err == nil {
		t.Fatal("expected error for gateway mode without URL")
	}

	// local mode with nonexistent binary should fail with "not found".
	_, err = b.Execute(t.Context(), "hello", ExecOptions{})
	if err == nil {
		t.Fatal("expected error for missing executable")
	}
	if !strings.Contains(err.Error(), "executable not found") {
		t.Errorf("expected 'executable not found', got: %v", err)
	}
}
```

- [ ] **Step 4: Run all zeroclaw tests**

```bash
cd /home/longwu/multica && go test ./server/pkg/agent/ -run "TestZeroclaw|TestBuildZeroclaw" -v -count=1
```

Expected: all tests pass.

- [ ] **Step 5: Verify compilation**

```bash
cd /home/longwu/multica && go build ./server/...
```

Expected: compiles successfully.

- [ ] **Step 6: Run full agent package tests to check no regressions**

```bash
cd /home/longwu/multica && go test ./server/pkg/agent/ -count=1 -timeout 120s
```

Expected: all tests pass (new + existing).

- [ ] **Step 7: Commit**

```bash
git add server/pkg/agent/zeroclaw.go server/pkg/agent/zeroclaw_test.go
git commit -m "feat(agent): implement zeroclaw gateway mode via WebSocket"
```
