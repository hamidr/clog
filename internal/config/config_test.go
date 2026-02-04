package config

import (
	"path/filepath"
	"strings"
	"testing"
)

// --- ProjectSlug ---

func TestProjectSlug_WhenGivenAbsolutePath_ShouldStripLeadingSlashAndReplaceSlashes(t *testing.T) {
	c := Config{LogBase: "/tmp/logs"}
	got := c.ProjectSlug("/Users/alice/projects/myapp")
	expected := "Users__alice__projects__myapp"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestProjectSlug_WhenGivenRootPath_ShouldReturnEmptyString(t *testing.T) {
	c := Config{LogBase: "/tmp/logs"}
	got := c.ProjectSlug("/")
	if got != "" {
		t.Errorf("expected empty string for root path, got %q", got)
	}
}

func TestProjectSlug_WhenGivenSingleDirectory_ShouldReturnNameWithoutSlash(t *testing.T) {
	c := Config{LogBase: "/tmp/logs"}
	got := c.ProjectSlug("/tmp")
	if got != "tmp" {
		t.Errorf("expected %q, got %q", "tmp", got)
	}
}

func TestProjectSlug_WhenGivenDeeplyNestedPath_ShouldReplaceAllSlashes(t *testing.T) {
	c := Config{LogBase: "/tmp/logs"}
	got := c.ProjectSlug("/a/b/c/d/e")
	expected := "a__b__c__d__e"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestProjectSlug_WhenPathContainsSpecialCharacters_ShouldPreserveThem(t *testing.T) {
	c := Config{LogBase: "/tmp/logs"}
	got := c.ProjectSlug("/home/user/my-project.v2")
	expected := "home__user__my-project.v2"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestProjectSlug_WhenPathContainsSpaces_ShouldPreserveThem(t *testing.T) {
	c := Config{LogBase: "/tmp/logs"}
	got := c.ProjectSlug("/home/user/My Project")
	expected := "home__user__My Project"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

// --- LogDir ---

func TestLogDir_ShouldCombineLogBaseAndProjectSlug(t *testing.T) {
	c := Config{LogBase: "/tmp/logs"}
	got := c.LogDir("/home/user/project")
	expected := filepath.Join("/tmp/logs", "home__user__project")
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

// --- DBPath ---

func TestDBPath_ShouldAppendEventsDuckdbToLogDir(t *testing.T) {
	c := Config{LogBase: "/tmp/logs"}
	got := c.DBPath("/home/user/project")

	if !strings.HasSuffix(got, "events.duckdb") {
		t.Errorf("expected path ending in events.duckdb, got %q", got)
	}

	expectedDir := filepath.Join("/tmp/logs", "home__user__project")
	if !strings.HasPrefix(got, expectedDir) {
		t.Errorf("expected path starting with %q, got %q", expectedDir, got)
	}
}

func TestDBPath_ShouldMatchLogDirPlusFilename(t *testing.T) {
	c := Config{LogBase: "/var/data"}
	got := c.DBPath("/foo/bar")
	expected := filepath.Join(c.LogDir("/foo/bar"), "events.duckdb")
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

// --- Default ---

func TestDefault_ShouldReturnLogBaseUnderHome(t *testing.T) {
	c := Default()
	if !strings.Contains(c.LogBase, ".claude") {
		t.Errorf("expected LogBase to contain .claude, got %q", c.LogBase)
	}
	if !strings.HasSuffix(c.LogBase, filepath.Join(".claude", "logs")) {
		t.Errorf("expected LogBase to end with .claude/logs, got %q", c.LogBase)
	}
}
