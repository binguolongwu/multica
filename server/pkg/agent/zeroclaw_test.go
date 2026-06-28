package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
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

		notif := `{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"s1","update":{"sessionUpdate":"tool_call_update","toolCallId":"call_1","name":"shell","title":"shell","kind":"bash","status":"completed","rawOutput":"/home/user\n","body":"/home/user\n"}}}`
		client.handleLine(notif)

		if gotMsg.Type != MessageToolResult || gotMsg.Tool != "shell" || gotMsg.Output != "/home/user\n" {
			t.Errorf("expected tool_result, got type=%q tool=%q output=%q", gotMsg.Type, gotMsg.Tool, gotMsg.Output)
		}
	})

	t.Run("parses agent_thought_chunk", func(t *testing.T) {
		var gotMsg Message
		client.onMessage = func(m Message) { gotMsg = m }

		notif := `{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"s1","update":{"sessionUpdate":"agent_thought_chunk","content":{"type":"text","text":"Let me think..."}}}}`
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

	// Simulate server pushing a response for a pending request.
	resp := zcRPCResponse{
		JSONRPC: "2.0",
		Result:  json.RawMessage(`{"sessionId":"abc123"}`),
		ID:      7,
	}
	data, _ := json.Marshal(resp)

	// Pre-load the pending map before calling handleLine.
	client.mu.Lock()
	ch := make(chan zcRPCResponse, 1)
	client.pending[7] = &zcPendingCall{response: ch}
	client.mu.Unlock()

	client.handleLine(string(data))

	select {
	case got := <-ch:
		var v struct {
			SessionID string `json:"sessionId"`
		}
		json.Unmarshal(got.Result, &v)
		if v.SessionID != "abc123" {
			t.Errorf("expected sessionId=abc123, got %q", v.SessionID)
		}
	default:
		t.Fatal("expected response to be dispatched to pending channel")
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


func TestZeroclawExecuteLocalBinaryNotFound(t *testing.T) {
	b := &zeroclawBackend{cfg: Config{
		ExecutablePath: "/nonexistent/zeroclaw",
	}}
	_, err := b.Execute(t.Context(), "hello", ExecOptions{})
	if err == nil {
		t.Fatal("expected error for missing executable")
	}
	if !strings.Contains(err.Error(), "executable not found") {
		t.Errorf("expected 'executable not found', got: %v", err)
	}
}


func TestZeroclawResolveGatewayURL(t *testing.T) {
	b := &zeroclawBackend{cfg: Config{
		Env: map[string]string{
			"ZEROCLAW_GATEWAY_URL":   "ws://192.168.1.100:42617",
			"ZEROCLAW_GATEWAY_TOKEN": "test-token-123",
		},
	}}

	url := b.resolveGatewayURL()
	if url != "ws://192.168.1.100:42617" {
		t.Errorf("expected gateway URL, got %q", url)
	}

	headers := b.gatewayHeaders()
	if got := headers.Get("Authorization"); got != "Bearer test-token-123" {
		t.Errorf("expected Bearer token, got %q", got)
	}
}

func TestZeroclawResolveGatewayURLMissing(t *testing.T) {
	b := &zeroclawBackend{cfg: Config{}}
	url := b.resolveGatewayURL()
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


func TestZeroclawIntegrationACP(t *testing.T) {
	execPath := "/home/longwu/.cargo/bin/zeroclaw"
	if _, err := exec.LookPath(execPath); err != nil {
		t.Skipf("zeroclaw not found at %s", execPath)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Start zeroclaw acp.
	cmd := exec.CommandContext(ctx, execPath, "acp")
	hideAgentWindow(cmd)
	cmd.Env = append(os.Environ(), "ZEROCLAW_UNSUPERVISED=1")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	cmd.Stderr = newLogWriter(slog.Default(), "[zeroclaw-int:stderr] ")

	if err := cmd.Start(); err != nil {
		t.Fatalf("start zeroclaw acp: %v", err)
	}
	defer func() {
		stdin.Close()
		cmd.Wait()
	}()

	client := newZeroclawClient(stdin)

	// Read stdout in background.
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
		for scanner.Scan() {
			client.handleLine(scanner.Text())
		}
		client.closeAllPending(fmt.Errorf("process exited"))
	}()

	// 1. Initialize
	initResult, err := client.request(ctx, "initialize", map[string]any{
		"protocolVersion": 1,
		"clientInfo": map[string]any{
			"name":    "multica-test",
			"version": "0.1.0",
		},
	})
	if err != nil {
		t.Fatalf("initialize failed: %v", err)
	}
	t.Logf("initialize response: %s", initResult)

	// Verify protocol version in response.
	var initResp struct {
		ProtocolVersion int `json:"protocolVersion"`
	}
	if err := json.Unmarshal(initResult, &initResp); err != nil {
		t.Fatalf("parse initialize response: %v", err)
	}
	if initResp.ProtocolVersion < 1 {
		t.Errorf("protocolVersion = %d, want >= 1", initResp.ProtocolVersion)
	}

	// 2. Create session.
	sessionResult, err := client.request(ctx, "session/new", map[string]any{
		"cwd":        "/tmp",
		"agentAlias": "default",
	})
	if err != nil {
		t.Fatalf("session/new failed: %v", err)
	}
	var sessionResp struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(sessionResult, &sessionResp); err != nil {
		t.Fatalf("parse session/new response: %v", err)
	}
	sessionID := sessionResp.SessionID
	if sessionID == "" {
		t.Fatal("session/new returned empty sessionId")
	}
	t.Logf("session created: %s", sessionID)

	// 3. Send prompt and collect streaming events.
	var (
		gotText   bool
		gotChunks []string
		msgMu     sync.Mutex
	)
	client.onMessage = func(m Message) {
		msgMu.Lock()
		defer msgMu.Unlock()
		if m.Type == MessageText {
			gotText = true
			gotChunks = append(gotChunks, m.Content)
		}
	}

	_, err = client.request(ctx, "session/prompt", map[string]any{
		"sessionId": sessionID,
		"prompt": []map[string]any{
			{"type": "text", "text": "Say exactly 'HELLO ZEROCLAW' and nothing else."},
		},
	})
	if err != nil {
		t.Fatalf("session/prompt failed: %v", err)
	}

	msgMu.Lock()
	hasText := gotText
	allChunks := strings.Join(gotChunks, "")
	msgMu.Unlock()

	if !hasText {
		t.Error("session/prompt did not produce any text chunks")
	} else {
		t.Logf("received text: %q", allChunks)
	}

	// 4. Stop session.
	_, err = client.request(ctx, "session/stop", map[string]any{
		"sessionId": sessionID,
	})
	if err != nil {
		t.Logf("session/stop returned error (non-fatal): %v", err)
	} else {
		t.Log("session/stop succeeded")
	}

	// Clean shutdown.
	stdin.Close()
	cancel()
	<-readerDone
}


func TestZeroclawIntegrationGateway(t *testing.T) {
	gatewayURL := os.Getenv("ZEROCLAW_TEST_GATEWAY_URL")
	if gatewayURL == "" {
		t.Skip("ZEROCLAW_TEST_GATEWAY_URL not set — skipping gateway integration test")
	}

	cfg := Config{
		Logger: slog.Default(),
		Env: map[string]string{
			"ZEROCLAW_GATEWAY_URL":   gatewayURL,
			"ZEROCLAW_GATEWAY_TOKEN": os.Getenv("ZEROCLAW_TEST_GATEWAY_TOKEN"),
		},
	}
	b := &zeroclawBackend{cfg: cfg}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	session, err := b.Execute(ctx, "Say exactly 'HELLO GW TEST' and nothing else.", ExecOptions{
		ZeroclawMode: "gateway",
	})
	if err != nil {
		t.Fatalf("Execute(gateway): %v", err)
	}

	var (
		gotText  bool
		allChunks []string
		mu       sync.Mutex
	)
	go func() {
		for msg := range session.Messages {
			mu.Lock()
			if msg.Type == MessageText {
				gotText = true
				allChunks = append(allChunks, msg.Content)
			}
			mu.Unlock()
		}
	}()

	result := <-session.Result
	mu.Lock()
	hasText := gotText
	fullText := strings.Join(allChunks, "")
	mu.Unlock()

	t.Logf("status=%s output=%q session_id=%s duration=%dms error=%s",
		result.Status, result.Output, result.SessionID, result.DurationMs, result.Error)

	if result.Status != "completed" {
		t.Errorf("expected status=completed, got %s", result.Status)
	}
	if !hasText {
		t.Error("no text chunks received from gateway")
	} else {
		t.Logf("received chunks: %q", fullText)
	}
}

// ── helpers ──

func testLogger() *slog.Logger {
	return slog.Default()
}

type nopWriteCloser struct{}

func (n *nopWriteCloser) Write(p []byte) (int, error) { return len(p), nil }
func (n *nopWriteCloser) Close() error                 { return nil }
