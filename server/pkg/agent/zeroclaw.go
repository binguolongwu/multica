package agent

import (
	"context"
	"fmt"
)

// zeroclawBlockedArgs are flags hardcoded by the daemon that must not be
// overridden by user-configured custom_args.
var zeroclawBlockedArgs = map[string]blockedArgMode{
	"acp":     blockedStandalone, // local mode must use acp protocol
	"-a":      blockedWithValue,  // agent alias managed by multica
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
