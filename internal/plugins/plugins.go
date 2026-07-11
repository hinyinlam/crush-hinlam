// Package plugins provides Claude Code plugin installation and discovery.
// Plugins are git repositories containing a .claude-plugin/plugin.json manifest
// and optionally a skills/ directory with SKILL.md files.
package plugins

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/crush/internal/home"
)

// PluginManifest represents the .claude-plugin/plugin.json file format.
type PluginManifest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Homepage    string `json:"homepage,omitempty"`
	Repository  string `json:"repository,omitempty"`
	License     string `json:"license,omitempty"`
}

// InstalledPlugin represents a plugin that has been installed locally.
type InstalledPlugin struct {
	Manifest PluginManifest
	Path     string // installation directory
	GitURL   string // original git URL
}

// PluginsDir returns the directory where plugins are installed.
func PluginsDir() string {
	return filepath.Join(home.Config(), "crush", "plugins")
}

// ParseRepoShorthand converts shorthand like "obra/superpowers" or
// "https://github.com/obra/superpowers" into a full git clone URL and a
// local directory name.
func ParseRepoShorthand(repo string) (gitURL, name string, err error) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return "", "", fmt.Errorf("repository cannot be empty")
	}

	// Already a full URL
	if strings.HasPrefix(repo, "https://") || strings.HasPrefix(repo, "git@") {
		gitURL = repo
		parts := strings.Split(repo, "/")
		name = strings.TrimSuffix(parts[len(parts)-1], ".git")
		return gitURL, name, nil
	}

	// Shorthand: owner/repo → github.com
	if strings.Contains(repo, "/") {
		gitURL = "https://github.com/" + repo
		parts := strings.Split(repo, "/")
		name = parts[len(parts)-1]
		return gitURL, name, nil
	}

	return "", "", fmt.Errorf("invalid repository format: %s (use owner/repo or full URL)", repo)
}

// Install clones a plugin repository and registers its skills directory.
// Returns the installation path and parsed manifest.
func Install(repo string) (*InstalledPlugin, error) {
	gitURL, name, err := ParseRepoShorthand(repo)
	if err != nil {
		return nil, err
	}

	pluginsDir := PluginsDir()
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create plugins directory: %w", err)
	}

	dest := filepath.Join(pluginsDir, name)

	// Check if already installed
	if _, err := os.Stat(dest); err == nil {
		// Try git pull to update
		pullCmd := exec.Command("git", "-C", dest, "pull", "--ff-only")
		_ = pullCmd.Run()
	} else {
		// Clone fresh
		cloneCmd := exec.Command("git", "clone", "--depth", "1", gitURL, dest)
		if out, err := cloneCmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("git clone failed: %w\n%s", err, string(out))
		}
	}

	// Parse manifest
	manifest, err := readManifest(dest)
	if err != nil {
		// Manifest is optional — plugin might just have skills/
		manifest = &PluginManifest{Name: name}
	}

	return &InstalledPlugin{
		Manifest: *manifest,
		Path:     dest,
		GitURL:   gitURL,
	}, nil
}

// readManifest reads and parses .claude-plugin/plugin.json from a directory.
func readManifest(dir string) (*PluginManifest, error) {
	manifestPath := filepath.Join(dir, ".claude-plugin", "plugin.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}
	var m PluginManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to parse plugin.json: %w", err)
	}
	return &m, nil
}

// ListInstalled returns all installed plugins in the plugins directory.
func ListInstalled() ([]InstalledPlugin, error) {
	pluginsDir := PluginsDir()
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var plugins []InstalledPlugin
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(pluginsDir, entry.Name())
		manifest, err := readManifest(path)
		if err != nil {
			manifest = &PluginManifest{Name: entry.Name()}
		}
		plugins = append(plugins, InstalledPlugin{
			Manifest: *manifest,
			Path:     path,
		})
	}
	return plugins, nil
}

// SkillsDirs returns the skills/ directory paths for all installed plugins.
// Only directories that actually exist are returned.
func SkillsDirs() []string {
	pluginsDir := PluginsDir()
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return nil
	}

	var dirs []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillsPath := filepath.Join(pluginsDir, entry.Name(), "skills")
		if info, err := os.Stat(skillsPath); err == nil && info.IsDir() {
			dirs = append(dirs, skillsPath)
		}
	}
	return dirs
}

// IsInstalled checks whether a plugin with the given name is installed.
func IsInstalled(name string) bool {
	path := filepath.Join(PluginsDir(), name)
	_, err := os.Stat(path)
	return err == nil
}
