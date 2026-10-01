package herdr

import (
	"context"
	"testing"
	"time"
)

func TestHerdrListAgents(t *testing.T) {
	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	agents, err := client.ListAgents(ctx)
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}

	t.Logf("Found %d agents", len(agents))
	for _, a := range agents {
		t.Logf("Agent: %s | Pane: %s | Status: %s | Cwd: %s", a.Agent, a.PaneID, a.AgentStatus, a.Cwd)
	}
}

func TestHerdrListWorkspaces(t *testing.T) {
	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	workspaces, err := client.ListWorkspaces(ctx)
	if err != nil {
		t.Fatalf("ListWorkspaces failed: %v", err)
	}

	t.Logf("Found %d workspaces", len(workspaces))
	for _, w := range workspaces {
		t.Logf("Workspace: %s | Label: %s | Status: %s", w.WorkspaceID, w.Label, w.AgentStatus)
	}
}
