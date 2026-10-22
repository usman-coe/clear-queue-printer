package spooler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClearDirRemovesFilesKeepsFolders(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"00001.SPL", "00001.SHD"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("job"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "keep"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := clearDir(dir); err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != "keep" {
		t.Fatalf("expected only the keep folder left, got %v", entries)
	}
}

// Smoke test for the PRINTER_INFO_2 struct layout: garbage names or a crash mean it is wrong.
func TestListReadsInstalledPrinters(t *testing.T) {
	ps, err := List()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range ps {
		t.Logf("%+v", p)
	}
}
