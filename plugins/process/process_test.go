package process

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mld.com/dtop/pkg/types"
)

func keyRunes(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestProcessFilterEditApply(t *testing.T) {
	p := New()

	p.Update(keyRunes("f"))
	if p.mode != modeFilterEdit {
		t.Fatalf("expected filter edit mode, got %v", p.mode)
	}

	p.Update(keyRunes("nginx"))
	p.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if p.mode != modeList {
		t.Fatalf("expected list mode after apply, got %v", p.mode)
	}
	if p.cfg.FilterString != "nginx" {
		t.Fatalf("expected filter to be applied, got %q", p.cfg.FilterString)
	}
}

func TestProcessFilterEditCursorDelete(t *testing.T) {
	p := New()

	p.Update(keyRunes("f"))
	p.Update(keyRunes("abc"))
	p.Update(tea.KeyMsg{Type: tea.KeyLeft})
	p.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	p.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if p.cfg.FilterString != "ac" {
		t.Fatalf("expected edited filter ac, got %q", p.cfg.FilterString)
	}
}

func TestProcessDetailModeEnterEsc(t *testing.T) {
	p := New()
	p.lastProcesses = []types.ProcessInfo{{PID: 123, Command: "sleep"}}
	p.selectedPID = 123

	p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if p.mode != modeDetail {
		t.Fatalf("expected detail mode, got %v", p.mode)
	}

	p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if p.mode != modeList {
		t.Fatalf("expected list mode after esc, got %v", p.mode)
	}
}

func TestProcessSignalChooserMode(t *testing.T) {
	p := New()
	p.lastProcesses = []types.ProcessInfo{{PID: 123, Command: "sleep"}}
	p.selectedPID = 123

	p.Update(keyRunes("x"))
	if p.mode != modeSignalChooser {
		t.Fatalf("expected signal chooser mode, got %v", p.mode)
	}
	if processSignals[p.signalIndex].name != "SIGTERM" {
		t.Fatalf("expected default SIGTERM selection")
	}

	p.Update(tea.KeyMsg{Type: tea.KeyDown})
	if p.signalIndex == 0 {
		t.Fatalf("expected signal index to move down")
	}

	p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if p.mode != modeList {
		t.Fatalf("expected list mode after cancel, got %v", p.mode)
	}
}
