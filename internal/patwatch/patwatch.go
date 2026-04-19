// Package patwatch keeps the ZITADEL provisioner PAT in memory and
// reloads it from disk whenever the operator rotates the underlying
// secret — without requiring a process restart.
//
// Why a watcher instead of one-shot read at startup:
//
//   - Kubernetes Secret rotation atomically swaps the file/symlink
//     under the mount path; today cmd/app would only see the new PAT
//     after a kubectl rollout restart, which the K4 scenario in
//     docs/operations.md acknowledges as MVP debt.
//   - Local dev `mise run infra:reset && mise run infra` overwrites
//     .secrets/zitadel-provisioner.pat; restart is friction.
//
// Design rules:
//
//   - last-known-good: an empty/missing/unreadable update never wipes
//     the previous valid PAT. Only a successful read of a non-empty,
//     trimmed value replaces it.
//   - directory-level watch: Kubernetes mounts a Secret as a symlink
//     into ..data/, then atomically swaps the symlink. Watching only
//     the target file misses these renames, so the watcher subscribes
//     to the parent directory and filters events by basename.
//   - startup is fail-closed: if the initial PAT cannot be read or is
//     blank, New returns an error and the app does not start.
package patwatch

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
)

// Watcher loads a PAT from a file and keeps it fresh as the file is
// rewritten or symlink-swapped.
type Watcher struct {
	path     string
	dir      string
	basename string
	current  atomic.Pointer[string]
	fs       *fsnotify.Watcher
	logger   *slog.Logger
	done     chan struct{}
}

// New reads the initial PAT from path and starts watching the parent
// directory for rotation events. Returns an error if the file is
// missing, unreadable, or blank — startup is fail-closed.
//
// The returned Watcher must be Close()d when the app shuts down.
func New(path string, logger *slog.Logger) (*Watcher, error) {
	if path == "" {
		return nil, errors.New("patwatch: path is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("patwatch: resolve %q: %w", path, err)
	}

	initial, err := readTrimmed(abs)
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		path:     abs,
		dir:      filepath.Dir(abs),
		basename: filepath.Base(abs),
		logger:   logger,
		done:     make(chan struct{}),
	}
	w.current.Store(&initial)

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("patwatch: create fsnotify watcher: %w", err)
	}
	// Watch the parent directory, not the file itself — Kubernetes
	// rotates Secrets by atomically swapping the symlink, which a
	// file-level watch would miss.
	if err := fsw.Add(w.dir); err != nil {
		_ = fsw.Close()
		return nil, fmt.Errorf("patwatch: watch %q: %w", w.dir, err)
	}
	w.fs = fsw

	go w.loop()
	return w, nil
}

// Get returns the current PAT. Always non-empty after a successful
// New() — last-known-good semantics guarantee a blank or invalid
// later read never replaces the value.
func (w *Watcher) Get() string {
	if p := w.current.Load(); p != nil {
		return *p
	}
	return ""
}

// Close stops the watcher and releases the fsnotify handle.
func (w *Watcher) Close() error {
	close(w.done)
	if w.fs == nil {
		return nil
	}
	return w.fs.Close()
}

func (w *Watcher) loop() {
	for {
		select {
		case <-w.done:
			return
		case ev, ok := <-w.fs.Events:
			if !ok {
				return
			}
			// Filter to events that touch the basename we care
			// about. Compare on basename only — Kubernetes paths
			// like /etc/.../basename and /etc/.../..data/basename
			// both surface here.
			if filepath.Base(ev.Name) != w.basename {
				continue
			}
			// Any of these can be the moment a rotation completes.
			// fsnotify Op is a bitmask, so use Has().
			interesting := ev.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Rename|fsnotify.Chmod|fsnotify.Remove) != 0
			if !interesting {
				continue
			}
			w.reload(ev)
		case err, ok := <-w.fs.Errors:
			if !ok {
				return
			}
			w.logger.Warn("patwatch: fsnotify error", "error", err)
		}
	}
}

func (w *Watcher) reload(ev fsnotify.Event) {
	next, err := readTrimmed(w.path)
	if err != nil {
		// Could be the gap between Remove and Create during a
		// rotation. Keep the last-known-good value and move on.
		w.logger.Warn(
			"patwatch: re-read failed during rotation; keeping previous PAT",
			"event", ev.Op.String(),
			"error", err,
		)
		return
	}
	prev := w.current.Load()
	if prev != nil && *prev == next {
		return
	}
	w.current.Store(&next)
	w.logger.Info("patwatch: PAT reloaded", "event", ev.Op.String())
}

// readTrimmed reads path, trims whitespace, and rejects blank
// content. Used both at startup (fail-closed) and at every rotation
// (last-known-good).
func readTrimmed(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("patwatch: file %q does not exist", path)
		}
		return "", fmt.Errorf("patwatch: read %q: %w", path, err)
	}
	v := strings.TrimSpace(string(raw))
	if v == "" {
		return "", fmt.Errorf("patwatch: file %q is empty", path)
	}
	return v, nil
}
