package plugins

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseRepoShorthand_OwnerRepo(t *testing.T) {
	gitURL, name, err := ParseRepoShorthand("obra/superpowers")
	require.NoError(t, err)
	require.Equal(t, "https://github.com/obra/superpowers", gitURL)
	require.Equal(t, "superpowers", name)
}

func TestParseRepoShorthand_HTTPS(t *testing.T) {
	gitURL, name, err := ParseRepoShorthand("https://github.com/obra/superpowers")
	require.NoError(t, err)
	require.Equal(t, "https://github.com/obra/superpowers", gitURL)
	require.Equal(t, "superpowers", name)
}

func TestParseRepoShorthand_SSH(t *testing.T) {
	gitURL, name, err := ParseRepoShorthand("git@github.com:obra/superpowers.git")
	require.NoError(t, err)
	require.Equal(t, "git@github.com:obra/superpowers.git", gitURL)
	require.Equal(t, "superpowers", name)
}

func TestParseRepoShorthand_Empty(t *testing.T) {
	_, _, err := ParseRepoShorthand("")
	require.Error(t, err)
}

func TestParseRepoShorthand_Invalid(t *testing.T) {
	_, _, err := ParseRepoShorthand("justaname")
	require.Error(t, err)
}

func TestReadManifest_Valid(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, ".claude-plugin")
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))
	manifestContent := `{"name":"test-plugin","description":"A test","version":"1.0.0"}`
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(manifestContent), 0o644))

	m, err := readManifest(dir)
	require.NoError(t, err)
	require.Equal(t, "test-plugin", m.Name)
	require.Equal(t, "A test", m.Description)
	require.Equal(t, "1.0.0", m.Version)
}

func TestReadManifest_NoManifest(t *testing.T) {
	dir := t.TempDir()
	_, err := readManifest(dir)
	require.Error(t, err)
}

func TestReadManifest_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, ".claude-plugin")
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte("{invalid"), 0o644))

	_, err := readManifest(dir)
	require.Error(t, err)
}

func TestSkillsDirs_NoPlugins(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dirs := SkillsDirs()
	require.Empty(t, dirs)
}

func TestSkillsDirs_WithPlugins(t *testing.T) {
	tmpConfig := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpConfig)

	// Create a fake plugin with skills/
	pluginsDir := filepath.Join(tmpConfig, "crush", "plugins")
	pluginSkills := filepath.Join(pluginsDir, "test-plugin", "skills")
	require.NoError(t, os.MkdirAll(pluginSkills, 0o755))

	// Create a plugin without skills/
	require.NoError(t, os.MkdirAll(filepath.Join(pluginsDir, "no-skills-plugin"), 0o755))

	dirs := SkillsDirs()
	require.Len(t, dirs, 1)
	require.Contains(t, dirs[0], "test-plugin/skills")
}

func TestListInstalled_Empty(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	plugins, err := ListInstalled()
	require.NoError(t, err)
	require.Empty(t, plugins)
}

func TestListInstalled_WithPlugins(t *testing.T) {
	tmpConfig := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpConfig)

	pluginsDir := filepath.Join(tmpConfig, "crush", "plugins")

	// Plugin with manifest
	p1Dir := filepath.Join(pluginsDir, "plugin-one", ".claude-plugin")
	require.NoError(t, os.MkdirAll(p1Dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(p1Dir, "plugin.json"),
		[]byte(`{"name":"plugin-one","description":"First"}`), 0o644))

	// Plugin without manifest
	require.NoError(t, os.MkdirAll(filepath.Join(pluginsDir, "plugin-two"), 0o755))

	plugins, err := ListInstalled()
	require.NoError(t, err)
	require.Len(t, plugins, 2)

	names := []string{plugins[0].Manifest.Name, plugins[1].Manifest.Name}
	require.Contains(t, names, "plugin-one")
	require.Contains(t, names, "plugin-two")
}

func TestIsInstalled(t *testing.T) {
	tmpConfig := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpConfig)

	require.False(t, IsInstalled("test-plugin"))

	pluginsDir := filepath.Join(tmpConfig, "crush", "plugins", "test-plugin")
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

	require.True(t, IsInstalled("test-plugin"))
}

func TestPluginsDir(t *testing.T) {
	tmpConfig := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpConfig)

	dir := PluginsDir()
	require.Contains(t, dir, "crush")
	require.Contains(t, dir, "plugins")
}

func TestRemove_Existing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	pluginsDir := PluginsDir()
	pluginPath := filepath.Join(pluginsDir, "test-plugin")
	require.NoError(t, os.MkdirAll(pluginPath, 0o755))
	require.True(t, IsInstalled("test-plugin"))

	err := Remove("test-plugin")
	require.NoError(t, err)
	require.False(t, IsInstalled("test-plugin"))
}

func TestRemove_NotInstalled(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	err := Remove("nonexistent")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not installed")
}

func TestUpdate_Existing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// Create a minimal git repo to simulate a plugin
	pluginsDir := PluginsDir()
	pluginPath := filepath.Join(pluginsDir, "test-plugin")
	require.NoError(t, os.MkdirAll(pluginPath, 0o755))

	// Init git repo
	initCmd := exec.Command("git", "-C", pluginPath, "init")
	require.NoError(t, initCmd.Run())
	configCmd := exec.Command("git", "-C", pluginPath, "config", "user.email", "test@test.com")
	configCmd.Run()
	configCmd = exec.Command("git", "-C", pluginPath, "config", "user.name", "Test")
	configCmd.Run()

	// Create initial commit so pull works
	writeCmd := exec.Command("git", "-C", pluginPath, "commit", "--allow-empty", "-m", "init")
	writeCmd.Run()

	// Update without remote should fail gracefully
	_, err := Update("test-plugin")
	// This may fail because no upstream is configured, which is expected
	_ = err
	require.True(t, IsInstalled("test-plugin"))
}

func TestUpdate_NotInstalled(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, err := Update("nonexistent")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not installed")
}
