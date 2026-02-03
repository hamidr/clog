// Package config resolves filesystem paths for the session logger.
package config

import (
	"os"
	"path/filepath"
	"strings"
)

// Config holds base paths used by the logger.
type Config struct {
	LogBase string
}

// Default returns a Config rooted at ~/.claude/logs.
func Default() Config {
	return Config{
		LogBase: filepath.Join(os.Getenv("HOME"), ".claude", "logs"),
	}
}

// ProjectSlug converts a working directory into a safe directory name.
func (c Config) ProjectSlug(cwd string) string {
	slug := strings.TrimPrefix(cwd, "/")
	return strings.ReplaceAll(slug, "/", "__")
}

// LogDir returns the per-project log directory.
func (c Config) LogDir(cwd string) string {
	return filepath.Join(c.LogBase, c.ProjectSlug(cwd))
}

// DBPath returns the DuckDB database file path for a project.
func (c Config) DBPath(cwd string) string {
	return filepath.Join(c.LogDir(cwd), "events.duckdb")
}
