package agent

import (
	"encoding/json"
	"log/slog"
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

// ── helpers ──

func testLogger() *slog.Logger {
	return slog.Default()
}

type nopWriteCloser struct{}

func (n *nopWriteCloser) Write(p []byte) (int, error) { return len(p), nil }
func (n *nopWriteCloser) Close() error                 { return nil }
