package herdr

import (
	"testing"
)

func TestCleanTitleAndDisplayName(t *testing.T) {
	tests := []struct {
		title       string
		cwd         string
		expectedSub string
	}{
		{
			title:       "π > Roblox Asset Privacy and Spawning",
			cwd:         "/Users/akbar/Code/projects/roblox/keep-silent",
			expectedSub: "[omp] keep-silent: Roblox Asset Privacy and Spawning",
		},
		{
			title:       "π ⠸ Telegram Bot SSH Agent Integration",
			cwd:         "/Users/akbar/Code/projects/konek",
			expectedSub: "[omp] konek: Telegram Bot SSH Agent Integration",
		},
		{
			title:       "π ⠧ Membuat Bot Telegram Kontrol SSH",
			cwd:         "/Users/akbar/Code/projects/konek",
			expectedSub: "[omp] konek: Membuat Bot Telegram Kontrol SSH",
		},
	}

	for _, tt := range tests {
		a := &Agent{
			Agent:                 "omp",
			TerminalTitleStripped: tt.title,
			Cwd:                   tt.cwd,
			PaneID:                "w5:p1",
		}

		got := a.DisplayName()
		if got != tt.expectedSub {
			t.Errorf("DisplayName() = %q, want %q", got, tt.expectedSub)
		}
	}
}

func TestPaneDisplayName(t *testing.T) {
	p := &Pane{
		PaneID:                "wF:pR",
		Cwd:                   "/Users/akbar/Code/projects/rdns",
		TerminalTitleStripped: "akbar@AKBARs-MacBook-Air:~/Code/projects/rdns",
		Agent:                 "",
	}

	got := p.DisplayName()
	expected := "[shell] rdns"
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}
