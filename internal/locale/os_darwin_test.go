// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

package locale

import (
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"
)

func TestPlistLanguages(t *testing.T) {
	dir := t.TempDir()
	plist, err := os.ReadFile("testdata/GlobalPreferences.plist")
	if err != nil {
		t.Fatal(err)
	}
	write := func(name string, data []byte) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	valid := write("valid.plist", plist)
	link := filepath.Join(dir, "link.plist")
	if err := os.Symlink(valid, link); err != nil {
		t.Fatal(err)
	}
	// Sparse, so that the size check rather than the read is what rejects it.
	big := write("big.plist", nil)
	if err := os.Truncate(big, maxPlistSize+1); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(dir, "fifo.plist")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, path string
		want       []string
	}{
		{"valid", valid, testdataLanguages},
		{"symlink to valid", link, testdataLanguages},
		{"missing", filepath.Join(dir, "missing.plist"), nil},
		{"directory", dir, nil},
		{"empty", write("empty.plist", nil), nil},
		{"malformed", write("bad.plist", plist[:40]), nil},
		{"oversized", big, nil},
		{"fifo without a writer", fifo, nil},
	}
	for _, tt := range tests {
		// Each call runs in its own goroutine: opening a FIFO that nobody writes to would otherwise block
		// forever, and so would reading a huge file in full.
		done := make(chan []string, 1)
		go func() { done <- plistLanguages(tt.path) }()
		select {
		case got := <-done:
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("%s: plistLanguages = %q, want %q", tt.name, got, tt.want)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: plistLanguages did not return", tt.name)
		}
	}
}

func TestPlatformLanguages(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if got := platformLanguages(); got != nil {
		t.Errorf("no preferences file: platformLanguages = %q, want nil", got)
	}
	prefs := filepath.Join(home, "Library", "Preferences")
	if err := os.MkdirAll(prefs, 0o700); err != nil {
		t.Fatal(err)
	}
	plist, err := os.ReadFile("testdata/GlobalPreferences.plist")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(prefs, ".GlobalPreferences.plist"), plist, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := platformLanguages(); !reflect.DeepEqual(got, testdataLanguages) {
		t.Errorf("platformLanguages = %q, want %q", got, testdataLanguages)
	}
}
