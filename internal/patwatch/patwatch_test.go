package patwatch_test

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Gabrielbdd/gospa/internal/patwatch"
)

func discardLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// awaitGet polls the watcher's value until want appears or the
// deadline expires. fsnotify is async; sleeping a fixed duration
// either flakes (too short) or wastes test time (too long).
func awaitGet(t *testing.T, w *patwatch.Watcher, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if w.Get() == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("watcher.Get() = %q after deadline; want %q", w.Get(), want)
}

func writePAT(t *testing.T, path, value string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}

func TestNew_InitialReadValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pat")
	writePAT(t, path, "  initial-value  \n")

	w, err := patwatch.New(path, discardLog())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer w.Close()

	if got := w.Get(); got != "initial-value" {
		t.Errorf("Get() = %q; want trimmed initial-value", got)
	}
}

func TestNew_MissingFileFails(t *testing.T) {
	w, err := patwatch.New(filepath.Join(t.TempDir(), "absent"), discardLog())
	if err == nil {
		w.Close()
		t.Fatal("expected error for missing file")
	}
}

func TestNew_EmptyFileFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pat")
	writePAT(t, path, "   \n\t  \n")

	w, err := patwatch.New(path, discardLog())
	if err == nil {
		w.Close()
		t.Fatal("expected error for blank file")
	}
}

func TestWatcher_ReloadsOnOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pat")
	writePAT(t, path, "v1")

	w, err := patwatch.New(path, discardLog())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer w.Close()

	writePAT(t, path, "v2")
	awaitGet(t, w, "v2")
}

// TestWatcher_ReloadsOnRename simulates the Kubernetes Secret rotation
// pattern: a sibling file is created and atomically renamed over the
// target path. The watcher subscribes to the parent dir, so the
// CREATE/RENAME event on the basename must trigger a reload.
func TestWatcher_ReloadsOnRename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pat")
	writePAT(t, path, "v1")

	w, err := patwatch.New(path, discardLog())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer w.Close()

	staging := filepath.Join(dir, "pat.tmp")
	writePAT(t, staging, "v2-via-rename")
	if err := os.Rename(staging, path); err != nil {
		t.Fatalf("rename: %v", err)
	}

	awaitGet(t, w, "v2-via-rename")
}

func TestWatcher_KeepsLastGoodOnEmptyUpdate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pat")
	writePAT(t, path, "v1")

	w, err := patwatch.New(path, discardLog())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer w.Close()

	// Truncate to empty. fsnotify will fire WRITE; the watcher must
	// detect the blank read and keep the previous value rather than
	// nuke runtime auth.
	writePAT(t, path, "")

	// Give the watcher time to process the event without changing
	// state. We can't use awaitGet here because we expect Get() NOT
	// to change.
	time.Sleep(150 * time.Millisecond)
	if got := w.Get(); got != "v1" {
		t.Errorf("Get() = %q; want last-known-good v1", got)
	}

	// A subsequent valid write should still take effect.
	writePAT(t, path, "v2")
	awaitGet(t, w, "v2")
}

func TestWatcher_KeepsLastGoodOnTransientRemove(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pat")
	writePAT(t, path, "v1")

	w, err := patwatch.New(path, discardLog())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer w.Close()

	if err := os.Remove(path); err != nil {
		t.Fatalf("remove: %v", err)
	}
	// Watcher should keep v1 — file is missing, last-known-good
	// applies.
	time.Sleep(150 * time.Millisecond)
	if got := w.Get(); got != "v1" {
		t.Errorf("Get() = %q after remove; want last-known-good v1", got)
	}

	// Re-create with new value; watcher should pick it up.
	writePAT(t, path, "v2-after-recreate")
	awaitGet(t, w, "v2-after-recreate")
}
