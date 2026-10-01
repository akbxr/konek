package herdr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\].*?\x07|\x1b[PX^_].*?\x1b\\`)

// StripANSI removes terminal escape sequences from text.
func StripANSI(str string) string {
	return ansiRegex.ReplaceAllString(str, "")
}

type Client struct {
	binPath string
}

func NewClient(binPath ...string) *Client {
	bin := "herdr"
	if len(binPath) > 0 && binPath[0] != "" {
		bin = binPath[0]
	}
	return &Client{binPath: bin}
}

func (c *Client) execCommand(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, c.binPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		errOutput := strings.TrimSpace(stderr.String())
		if errOutput == "" {
			errOutput = strings.TrimSpace(stdout.String())
		}
		if errOutput == "" {
			errOutput = err.Error()
		}
		return nil, fmt.Errorf("herdr %s failed: %s", strings.Join(args, " "), errOutput)
	}
	return stdout.Bytes(), nil
}

// ListAgents lists all running agents recognized by Herdr.
func (c *Client) ListAgents(ctx context.Context) ([]Agent, error) {
	out, err := c.execCommand(ctx, "agent", "list")
	if err != nil {
		return nil, err
	}

	var resp AgentListResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse agent list JSON: %w", err)
	}
	return resp.Result.Agents, nil
}

// GetAgent retrieves the status and information of a specific agent.
func (c *Client) GetAgent(ctx context.Context, target string) (*Agent, error) {
	out, err := c.execCommand(ctx, "agent", "get", target)
	if err != nil {
		return nil, err
	}

	var resp AgentGetResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse agent get JSON: %w", err)
	}
	return &resp.Result.Agent, nil
}

// PromptAgent submits prompt text to the target agent.
func (c *Client) PromptAgent(ctx context.Context, target string, prompt string) error {
	_, err := c.execCommand(ctx, "agent", "prompt", target, prompt)
	return err
}

// ReadAgent returns the recent terminal output of the target agent.
func (c *Client) ReadAgent(ctx context.Context, target string, lines int) (string, error) {
	if lines <= 0 {
		lines = 60
	}
	out, err := c.execCommand(ctx, "agent", "read", target, "--source", "recent-unwrapped", "--lines", strconv.Itoa(lines))
	if err != nil {
		return "", err
	}
	return StripANSI(string(out)), nil
}

// SendKeys sends key presses to the target agent (e.g. "enter", "ctrl+c", "esc", "y", "n").
func (c *Client) SendKeys(ctx context.Context, target string, keys ...string) error {
	args := append([]string{"agent", "send-keys", target}, keys...)
	_, err := c.execCommand(ctx, args...)
	return err
}

// ListWorkspaces lists all workspaces in Herdr.
func (c *Client) ListWorkspaces(ctx context.Context) ([]Workspace, error) {
	out, err := c.execCommand(ctx, "workspace", "list")
	if err != nil {
		return nil, err
	}

	var resp WorkspaceListResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse workspace list JSON: %w", err)
	}
	return resp.Result.Workspaces, nil
}

// RunInPane runs a shell command inside a specific pane.
func (c *Client) RunInPane(ctx context.Context, paneID string, cmd string) error {
	_, err := c.execCommand(ctx, "pane", "run", paneID, cmd)
	return err
}

// FocusAgent focuses the given agent pane.
func (c *Client) FocusAgent(ctx context.Context, target string) error {
	_, err := c.execCommand(ctx, "agent", "focus", target)
	return err
}

// WaitForSettled waits for an agent to transition from working to a settled state (idle, done, blocked).
func (c *Client) WaitForSettled(ctx context.Context, target string, timeout time.Duration) (*Agent, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctxWithTimeout.Done():
			return nil, ctxWithTimeout.Err()
		case <-ticker.C:
			agent, err := c.GetAgent(ctxWithTimeout, target)
			if err != nil {
				continue
			}
			if agent.AgentStatus != "working" {
				return agent, nil
			}
		}
	}
}
