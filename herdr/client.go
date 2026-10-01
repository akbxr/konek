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

// ReadAgent returns recent output using agent read if available, falling back to pane read.
func (c *Client) ReadAgent(ctx context.Context, target string, lines int) (string, error) {
	if lines <= 0 {
		lines = 60
	}
	out, err := c.execCommand(ctx, "agent", "read", target, "--source", "recent-unwrapped", "--lines", strconv.Itoa(lines))
	if err != nil {
		return c.ReadPane(ctx, target, lines)
	}
	return StripANSI(string(out)), nil
}

// ReadPane returns the recent terminal output of any pane.
func (c *Client) ReadPane(ctx context.Context, paneID string, lines int) (string, error) {
	if lines <= 0 {
		lines = 60
	}
	out, err := c.execCommand(ctx, "pane", "read", paneID, "--source", "recent-unwrapped", "--lines", strconv.Itoa(lines))
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

// CreateWorkspace creates a new workspace with the given label and directory.
func (c *Client) CreateWorkspace(ctx context.Context, label string, cwd string) (*Workspace, *Pane, error) {
	args := []string{"workspace", "create"}
	if label != "" {
		args = append(args, "--label", label)
	}
	if cwd != "" {
		args = append(args, "--cwd", cwd)
	}
	args = append(args, "--no-focus")

	out, err := c.execCommand(ctx, args...)
	if err != nil {
		return nil, nil, err
	}

	var resp WorkspaceCreateResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, nil, fmt.Errorf("failed to parse workspace create JSON: %w", err)
	}
	return &resp.Result.Workspace, &resp.Result.RootPane, nil
}

// ListPanes lists all panes, optionally filtered by workspace ID.
func (c *Client) ListPanes(ctx context.Context, workspaceID ...string) ([]Pane, error) {
	args := []string{"pane", "list"}
	if len(workspaceID) > 0 && workspaceID[0] != "" {
		args = append(args, "--workspace", workspaceID[0])
	}

	out, err := c.execCommand(ctx, args...)
	if err != nil {
		return nil, err
	}

	var resp PaneListResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse pane list JSON: %w", err)
	}
	return resp.Result.Panes, nil
}

// SplitPane splits an existing pane in the specified direction ("right" or "down").
func (c *Client) SplitPane(ctx context.Context, paneID string, direction string) (*Pane, error) {
	if direction != "down" {
		direction = "right"
	}
	out, err := c.execCommand(ctx, "pane", "split", paneID, "--direction", direction, "--no-focus")
	if err != nil {
		return nil, err
	}

	var resp PaneSplitResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse pane split JSON: %w", err)
	}
	return &resp.Result.Pane, nil
}

// StartAgent launches a supported agent kind (omp, claude, codex, pi, etc.) in an existing shell pane.
func (c *Client) StartAgent(ctx context.Context, name string, kind string, paneID string, extraArgs ...string) error {
	args := []string{"agent", "start", name, "--kind", kind, "--pane", paneID}
	if len(extraArgs) > 0 {
		args = append(args, "--")
		args = append(args, extraArgs...)
	}
	_, err := c.execCommand(ctx, args...)
	return err
}

// CloseWorkspace closes a workspace by ID.
func (c *Client) CloseWorkspace(ctx context.Context, workspaceID string) error {
	_, err := c.execCommand(ctx, "workspace", "close", workspaceID)
	return err
}

// AbortTurn cancels only the active LLM generation/turn while keeping the agent process alive.
// In OMP, Claude Code, and Pi, Escape is the standard key to cancel a turn.
func (c *Client) AbortTurn(ctx context.Context, target string) error {
	err := c.SendKeys(ctx, target, "esc")
	if err != nil {
		_, err = c.execCommand(ctx, "pane", "send-keys", target, "esc")
	}
	return err
}

// SendInterrupt sends Ctrl+C (SIGINT) to the agent and pane terminal.
func (c *Client) SendInterrupt(ctx context.Context, target string) error {
	_ = c.SendKeys(ctx, target, "ctrl+c")
	_, err := c.execCommand(ctx, "pane", "send-keys", target, "ctrl+c")
	return err
}
