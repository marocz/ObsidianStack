package config

import (
	"context"
	"log/slog"

	"github.com/fsnotify/fsnotify"
)

// Watch monitors path for changes and calls onChange with the newly loaded
// Config each time the file is written. It runs until ctx is cancelled.
//
// If a reload fails (e.g., invalid YAML), the error is logged and the
// previous config remains active — Watch does not call onChange.
func Watch(ctx context.Context, path string, onChange func(*Config)) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	if err := watcher.Add(path); err != nil {
		return err
	}

	slog.Info("config: watching for changes", "path", path)

	for {
		select {
		case <-ctx.Done():
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			// Only reload on write or create events. Editors often write via
			// rename (atomic save), so also catch fsnotify.Create.
			if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
				continue
			}

			cfg, err := Load(path)
			if err != nil {
				slog.Error("config: reload failed — keeping previous config",
					"path", path, "err", err)
				continue
			}

			slog.Info("config: reloaded", "path", path)
			onChange(cfg)

			// Re-add the file in case an atomic save replaced the inode.
			_ = watcher.Add(path)

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			slog.Error("config: watcher error", "err", err)
		}
	}
}
