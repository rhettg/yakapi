package ci

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/rhettg/yakapi/internal/stream"
)

type CommandID string

type Result struct {
	ID     CommandID
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

type Command struct {
	ID   CommandID `json:"id"`
	Cmd  string    `json:"cmd"`
	Args string    `json:"args,omitempty"`
}

func Accept(ctx context.Context, sm *stream.Manager, cmdStr string) (CommandID, error) {
	if cmdStr == "" {
		return "", errors.New("empty command")
	}

	cmdID := CommandID(uuid.New().String())

	f := strings.Fields(cmdStr)

	cmd := Command{
		ID:   cmdID,
		Cmd:  f[0],
		Args: strings.Join(f[1:], " "),
	}

	err := streamCommand(ctx, sm, cmd)
	if err != nil {
		return "", err
	}

	return cmdID, nil
}

type ResultCollector struct {
	results [256]Result
	ndx     int

	mu sync.RWMutex
}

func (rc *ResultCollector) FetchResult(id CommandID) Result {
	if id == "" {
		return Result{}
	}

	rc.mu.RLock()
	defer rc.mu.RUnlock()

	for i := 0; i < len(rc.results); i++ {
		if rc.results[i].ID == id {
			return rc.results[i]
		}
	}

	return Result{}
}

func (rc *ResultCollector) Collect(ctx context.Context, sm *stream.Manager) error {
	s := sm.GetReader("ci:result")
	defer sm.ReturnReader("ci:result", s)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case data, ok := <-s:
			if !ok {
				return errors.New("stream closed")
			}
			var result Result
			err := json.Unmarshal([]byte(data), &result)
			if err != nil {
				slog.Warn("failed to unmarshal ci result", "error", err)
				continue
			}

			if result.ID == "" {
				slog.Warn("ci result missing id")
				continue
			}
			slog.Debug("collected ci result", "id", result.ID)

			rc.mu.Lock()
			rc.results[rc.ndx] = result
			rc.ndx = (rc.ndx + 1) % len(rc.results)
			rc.mu.Unlock()
		}
	}
}

func streamCommand(ctx context.Context, sm *stream.Manager, cmd Command) error {
	data, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to serialize command: %w", err)
	}

	s := sm.GetWriter("ci")
	defer sm.ReturnWriter("ci")

	select {
	case s <- data:
	case <-ctx.Done():
		return ctx.Err()
	}

	slog.Info("streamed command", "stream", "yakapi:ci", "id", cmd.ID)

	return nil
}
