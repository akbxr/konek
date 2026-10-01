package herdr

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var titlePrefixRegex = regexp.MustCompile(`^[\x{03c0}\x{03a0}\x{2800}-\x{28ff}\s>❯»\-_|*:]+`)

// CleanTitle strips terminal prefixes like "π > ", "π ⠸ ", etc.
func CleanTitle(title string) string {
	cleaned := titlePrefixRegex.ReplaceAllString(title, "")
	return strings.TrimSpace(cleaned)
}

// DisplayName returns a human-friendly title like "[omp] keep-silent: Roblox Asset Privacy and Spawning".
func (a *Agent) DisplayName() string {
	title := CleanTitle(a.TerminalTitleStripped)
	if title == "" {
		title = CleanTitle(a.TerminalTitle)
	}

	projectName := ""
	if a.Cwd != "" {
		projectName = filepath.Base(a.Cwd)
	}

	agentKind := a.Agent
	if agentKind == "" {
		agentKind = "agent"
	}

	if projectName != "" && title != "" {
		return fmt.Sprintf("[%s] %s: %s", agentKind, projectName, title)
	}
	if title != "" {
		return fmt.Sprintf("[%s] %s", agentKind, title)
	}
	if projectName != "" {
		return fmt.Sprintf("[%s] %s", agentKind, projectName)
	}
	return fmt.Sprintf("[%s] %s", agentKind, a.PaneID)
}
// Agent represents an agent returned by `herdr agent list` or `herdr agent get`.
type Agent struct {
	Agent                 string        `json:"agent"`
	AgentStatus           string        `json:"agent_status"`
	Cwd                   string        `json:"cwd"`
	ForegroundCwd         string        `json:"foreground_cwd"`
	Focused               bool          `json:"focused"`
	PaneID                string        `json:"pane_id"`
	TabID                 string        `json:"tab_id"`
	TerminalID            string        `json:"terminal_id"`
	TerminalTitle         string        `json:"terminal_title"`
	TerminalTitleStripped string        `json:"terminal_title_stripped"`
	WorkspaceID           string        `json:"workspace_id"`
	AgentSession          *AgentSession `json:"agent_session,omitempty"`
}

type AgentSession struct {
	Agent  string `json:"agent"`
	Kind   string `json:"kind"`
	Source string `json:"source"`
	Value  string `json:"value"`
}

type AgentListResponse struct {
	ID     string `json:"id"`
	Result struct {
		Agents []Agent `json:"agents"`
		Type   string  `json:"type"`
	} `json:"result"`
}

type AgentGetResponse struct {
	ID     string `json:"id"`
	Result struct {
		Agent Agent  `json:"agent"`
		Type  string `json:"type"`
	} `json:"result"`
}

type Workspace struct {
	ActiveTabID string `json:"active_tab_id"`
	AgentStatus string `json:"agent_status"`
	Focused     bool   `json:"focused"`
	Label       string `json:"label"`
	Number      int    `json:"number"`
	PaneCount   int    `json:"pane_count"`
	TabCount    int    `json:"tab_count"`
	WorkspaceID string `json:"workspace_id"`
}

type WorkspaceListResponse struct {
	ID     string `json:"id"`
	Result struct {
		Type       string      `json:"type"`
		Workspaces []Workspace `json:"workspaces"`
	} `json:"result"`
}
