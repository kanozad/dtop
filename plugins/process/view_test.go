package process

import (
	"strings"
	"testing"
	"time"

	"github.com/kanozad/dtop/internal/testutil"
	"github.com/kanozad/dtop/internal/theme"
	"github.com/kanozad/dtop/pkg/types"
)

func makeProcess(pid int, name string, cpu float64, memMB uint64) types.ProcessInfo {
	return types.ProcessInfo{
		PID:        pid,
		PPID:       1,
		User:       "user",
		State:      types.StateSleeping,
		Command:    name,
		FullCmd:    name,
		CPUPercent: cpu,
		MemBytes:   memMB * 1024 * 1024,
		StartTime:  time.Now().Add(-time.Minute),
	}
}

func makeStats(procs ...types.ProcessInfo) types.ProcessStats {
	return types.ProcessStats{
		Processes:  procs,
		TotalCount: len(procs),
		Timestamp:  time.Now(),
		SortBy:     types.SortByCPU,
		SortDesc:   true,
	}
}

func TestProcessViewNoData(t *testing.T) {
	p := New()
	th := theme.Default()
	out := p.View(nil, 80, 10, th)
	if !strings.Contains(out, "Processes") {
		t.Errorf("expected box title 'Processes', got: %q", out)
	}
	if !strings.Contains(out, "Collecting...") {
		t.Errorf("expected 'Collecting...' placeholder, got: %q", out)
	}
}

func TestProcessViewAtMinHeight(t *testing.T) {
	p := New()
	th := theme.Default()
	stats := makeStats(
		makeProcess(1, "init", 0.1, 10),
		makeProcess(100, "bash", 0.5, 20),
		makeProcess(200, "vim", 1.2, 50),
	)

	// At MinH=5: title(1) + header(1) + colHeader(1) + hr(1) + at_least_1_process(1) = 5.
	minH := p.SizeHint().MinH
	out := p.View(stats, 80, minH, th)
	if out == "" {
		t.Fatal("View returned empty output at MinH")
	}
	if !strings.Contains(out, "Processes") {
		t.Errorf("expected box title 'Processes', got: %q", out)
	}
	// At MinH there is room for exactly 1 process row.
	if !strings.Contains(out, "init") && !strings.Contains(out, "bash") && !strings.Contains(out, "vim") {
		t.Errorf("expected at least one process name at MinH, got: %q", out)
	}
}

func TestProcessViewShowsProcesses(t *testing.T) {
	p := New()
	th := theme.Default()
	stats := makeStats(
		makeProcess(1, "init", 0.1, 10),
		makeProcess(100, "bash", 0.5, 20),
		makeProcess(200, "vim", 1.2, 50),
	)

	// At h=10: title(1) + header(1) + colHeader(1) + hr(1) + 6 process rows = 10 content rows.
	out := p.View(stats, 80, 10, th)
	if !strings.Contains(out, "init") {
		t.Errorf("expected 'init' in process list, got: %q", out)
	}
	if !strings.Contains(out, "bash") {
		t.Errorf("expected 'bash' in process list, got: %q", out)
	}
	if !strings.Contains(out, "vim") {
		t.Errorf("expected 'vim' in process list, got: %q", out)
	}
}

func TestProcessViewScrollIndicator(t *testing.T) {
	p := New()
	th := theme.Default()

	// Create more processes than can fit at h=7.
	// At h=7: availableLines = (7-1) - 3 = 3. With 6 processes, scroll indicator appears.
	procs := make([]types.ProcessInfo, 6)
	for i := range procs {
		procs[i] = makeProcess(i+1, "proc", float64(i), 10)
	}
	stats := makeStats(procs...)

	out := p.View(stats, 80, 7, th)
	// Scroll indicator is appended to the header: "(1-N of M)"
	if !strings.Contains(out, "of 6") {
		t.Errorf("expected scroll indicator 'of 6' when processes overflow, got: %q", out)
	}
}

func TestProcessViewColumnHeader(t *testing.T) {
	p := New()
	th := theme.Default()
	stats := makeStats(makeProcess(1, "init", 0.1, 10))

	out := p.View(stats, 80, 10, th)
	// Column header should contain PID and CPU labels.
	if !strings.Contains(out, "PID") {
		t.Errorf("expected 'PID' in column header, got: %q", out)
	}
	if !strings.Contains(out, "CPU") {
		t.Errorf("expected 'CPU' in column header, got: %q", out)
	}
}

func TestProcessViewOutputFitsHeight(t *testing.T) {
	p := New()
	th := theme.Default()
	stats := makeStats(
		makeProcess(1, "init", 0.1, 10),
		makeProcess(2, "bash", 0.5, 20),
	)

	for _, h := range []int{5, 8, 12, 20} {
		out := p.View(stats, 80, h, th)
		lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
		vChrome := 2
		want := h + vChrome
		if len(lines) > want {
			t.Errorf("View(h=%d) produced %d lines, want <= %d", h, len(lines), want)
		}
	}
}

// Golden-file tests: capture full rendered output (ANSI stripped) and compare
// against stored fixtures in testdata/. Re-generate with: go test -run 'Golden' -update

func makeFixedStats(procs ...types.ProcessInfo) types.ProcessStats {
	return types.ProcessStats{
		Processes:     procs,
		TotalCount:    len(procs),
		FilteredCount: len(procs),
		SortBy:        types.SortByCPU,
		SortDesc:      true,
	}
}

func makeFixedProcess(pid int, ppid int, user, name string, cpu float64, memMB uint64, state types.ProcessState) types.ProcessInfo {
	return types.ProcessInfo{
		PID:        pid,
		PPID:       ppid,
		User:       user,
		State:      state,
		Command:    name,
		FullCmd:    name,
		CPUPercent: cpu,
		MemBytes:   memMB * 1024 * 1024,
		StartTime:  time.Time{}, // zero — not rendered in list view
	}
}

func TestProcessGoldenList(t *testing.T) {
	p := New()
	th := theme.Default()
	stats := makeFixedStats(
		makeFixedProcess(1, 0, "root", "systemd", 0.0, 10, types.StateSleeping),
		makeFixedProcess(100, 1, "root", "kthreadd", 0.1, 0, types.StateSleeping),
		makeFixedProcess(1234, 1, "alice", "bash", 0.5, 20, types.StateRunning),
		makeFixedProcess(5678, 1234, "alice", "vim", 2.3, 50, types.StateRunning),
	)
	out := p.View(stats, 80, 12, th)
	testutil.CheckGolden(t, "process_list", out)
}

func TestProcessGoldenScrollIndicator(t *testing.T) {
	p := New()
	th := theme.Default()
	procs := []types.ProcessInfo{
		makeFixedProcess(1, 0, "root", "systemd", 0.0, 10, types.StateSleeping),
		makeFixedProcess(2, 1, "root", "kthread", 0.1, 0, types.StateSleeping),
		makeFixedProcess(3, 1, "alice", "bash", 0.5, 20, types.StateRunning),
		makeFixedProcess(4, 3, "alice", "vim", 2.3, 50, types.StateRunning),
		makeFixedProcess(5, 1, "bob", "python3", 5.0, 120, types.StateRunning),
		makeFixedProcess(6, 1, "bob", "node", 1.2, 80, types.StateSleeping),
	}
	stats := makeFixedStats(procs...)
	// h=7: 3 process rows visible, scroll indicator shown.
	out := p.View(stats, 80, 7, th)
	testutil.CheckGolden(t, "process_scroll", out)
}
