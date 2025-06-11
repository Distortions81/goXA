package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafeJoin(t *testing.T) {
	base := filepath.Join(t.TempDir(), "base")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	cases := []struct {
		target  string
		want    string
		wantErr bool
	}{
		{"file.txt", filepath.Join(base, "file.txt"), false},
		{"sub/dir/../file.txt", filepath.Join(base, "sub", "file.txt"), false},
		{filepath.Join("sub", "dir"), filepath.Join(base, "sub", "dir"), false},
		{"../../evil", "", true},
		{"..", "", true},
		{"../evil.txt", "", true},
		{"/../../evil", filepath.Join(base, "evil"), false},
		{"/absolute/file", filepath.Join(base, "absolute", "file"), false},
	}

	for _, tc := range cases {
		got, err := safeJoin(base, tc.target)
		if tc.wantErr {
			if err == nil {
				t.Errorf("expected error for target %q, got path %q", tc.target, got)
			}
		} else {
			if err != nil {
				t.Errorf("unexpected error for target %q: %v", tc.target, err)
			} else if got != tc.want {
				t.Errorf("safeJoin(%q) = %q, want %q", tc.target, got, tc.want)
			}
		}
	}
}
