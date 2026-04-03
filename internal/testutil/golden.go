// Package testutil provides shared helpers for golden-file tests across plugins.
package testutil

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// UpdateGolden controls whether golden files are re-written instead of compared.
// Set by passing -update to go test, e.g.:
//
//	go test ./plugins/cpu/ -update
var UpdateGolden = flag.Bool("update", false, "update golden test fixtures")

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// StripANSI removes ANSI terminal escape sequences from s.
func StripANSI(s string) string {
	return ansiEscape.ReplaceAllString(s, "")
}

// CheckGolden compares got (with ANSI stripped) against a golden file stored in
// testdata/<name>.golden relative to the test's working directory. If -update is
// set, the file is written instead of compared. Missing golden files are a test
// failure with a hint to run with -update.
func CheckGolden(t *testing.T, name, got string) {
	t.Helper()
	clean := StripANSI(got)
	path := filepath.Join("testdata", name+".golden")
	if *UpdateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(path, []byte(clean), 0o644); err != nil {
			t.Fatalf("writing golden file %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden file %s missing — run: go test -run %s -update\nerr: %v", path, name, err)
	}
	if clean != string(want) {
		t.Errorf("golden mismatch for %q\n--- want ---\n%s\n--- got ---\n%s", name, want, clean)
	}
}
