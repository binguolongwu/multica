package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// zeroclawBlockedArgs are flags hardcoded by the daemon that must not be
// overridden by user-configured custom_args.
var zeroclawBlockedArgs = map[string]blockedArgMode{
	"acp":     blockedStandalone, // local mode must use acp protocol
	"-a":      blockedWithValue,  // agent alias managed by multica
	"--alias": blockedWithValue,
}

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
	SessionID string      `json:"sessionId"`
	Update    zcACPUpdate `json:"update"`
}

// zcACPPromptResult is the success response from session/prompt.
type zcACPPromptResult struct {
	StopReason string `json:"stopReason"`
}

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
		nextID:  1,
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
	if err := json.Unmarshal([]byte(line), &resp); err == nil && resp.JSONRPC == "2.0" && (resp.Error != nil || resp.Result != nil) {
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

// ── Backend ──────────────────────────────────────────────────────

// zeroclawBackend implements Backend by spawning `zeroclaw acp` (local mode)
// or connecting to a zeroclaw gateway via WebSocket (gateway mode).
type zeroclawBackend struct {
	cfg Config
}

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

func (b *zeroclawBackend) executeGateway(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	gatewayURL := b.resolveGatewayURL()
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
		conn, _, err := websocket.DefaultDialer.DialContext(ctx, gatewayURL, b.gatewayHeaders())
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
func (b *zeroclawBackend) resolveGatewayURL() string {
	return b.cfg.Env["ZEROCLAW_GATEWAY_URL"]
}

// gatewayHeaders builds HTTP headers for the gateway WebSocket handshake,
// including an optional bearer token from custom_env.
func (b *zeroclawBackend) gatewayHeaders() http.Header {
	h := http.Header{}
	if token := b.cfg.Env["ZEROCLAW_GATEWAY_TOKEN"]; token != "" {
		h.Set("Authorization", "Bearer "+token)
	}
	return h
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
		_, err := client.request(runCtx, "initialize", map[string]any{
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
