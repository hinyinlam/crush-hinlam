package plugins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestInstallPipeline_Simulated simulates the full install pipeline:
// repo parse → directory creation → manifest reading → skills discovery.
// This is an integration test that doesn't require network access.
func TestInstallPipeline_Simulated(t *testing.T) {
	tmpConfig := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpConfig)

	// Step 1: Parse repo shorthand
	gitURL, name, err := ParseRepoShorthand("obra/superpowers")
	require.NoError(t, err)
	require.Equal(t, "https://github.com/obra/superpowers", gitURL)
	require.Equal(t, "superpowers", name)

	// Step 2: Create the plugin directory structure (simulating git clone)
	pluginsDir := PluginsDir()
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))
	pluginDir := filepath.Join(pluginsDir, name)
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))

	// Step 3: Write a manifest
	manifestDir := filepath.Join(pluginDir, ".claude-plugin")
	require.NoError(t, os.MkdirAll(manifestDir, 0o755))
	manifestJSON := `{"name":"superpowers","description":"Skills library","version":"6.1.1"}`
	require.NoError(t, os.WriteFile(filepath.Join(manifestDir, "plugin.json"),
		[]byte(manifestJSON), 0o644))

	// Step 4: Write skills
	skillsDir := filepath.Join(pluginDir, "skills", "test-driven-development")
	require.NoError(t, os.MkdirAll(skillsDir, 0o755))
	skillContent := "---\nname: test-driven-development\ndescription: Use when implementing features\n---\n# TDD\nWrite tests first."
	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "SKILL.md"),
		[]byte(skillContent), 0o644))

	// Step 5: Verify discovery
	require.True(t, IsInstalled("superpowers"))

	installed, err := ListInstalled()
	require.NoError(t, err)
	require.Len(t, installed, 1)
	require.Equal(t, "superpowers", installed[0].Manifest.Name)
	require.Equal(t, "6.1.1", installed[0].Manifest.Version)

	dirs := SkillsDirs()
	require.Len(t, dirs, 1)
	require.Contains(t, dirs[0], "superpowers/skills")

	// Step 6: Verify the SKILL.md file exists at the discovered path
	skillFile := filepath.Join(dirs[0], "test-driven-development", "SKILL.md")
	data, err := os.ReadFile(skillFile)
	require.NoError(t, err)
	require.Contains(t, string(data), "name: test-driven-development")
}

// TestInstallPipeline_MultiplePlugins simulates installing both
// superpowers and caveman and verifying all skills are discovered.
func TestInstallPipeline_MultiplePlugins(t *testing.T) {
	tmpConfig := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpConfig)

	pluginsDir := PluginsDir()

	for _, plugin := range []struct {
		name, manifest string
		skills         []string
	}{
		{
			name:     "superpowers",
			manifest: `{"name":"superpowers","version":"6.1.1","description":"TDD skills"}`,
			skills:   []string{"brainstorming", "test-driven-development", "systematic-debugging"},
		},
		{
			name:     "caveman",
			manifest: `{"name":"caveman","description":"Token compression"}`,
			skills:   []string{"caveman", "caveman-commit"},
		},
	} {
		dir := filepath.Join(pluginsDir, plugin.name)
		manifestDir := filepath.Join(dir, ".claude-plugin")
		require.NoError(t, os.MkdirAll(manifestDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(manifestDir, "plugin.json"),
			[]byte(plugin.manifest), 0o644))

		for _, skill := range plugin.skills {
			skillDir := filepath.Join(dir, "skills", skill)
			require.NoError(t, os.MkdirAll(skillDir, 0o755))
			content := "---\nname: " + skill + "\ndescription: A skill\n---\n# " + skill
			require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
				[]byte(content), 0o644))
		}
	}

	// Verify both plugins discovered
	installed, err := ListInstalled()
	require.NoError(t, err)
	require.Len(t, installed, 2)

	// Verify skills dirs discovered
	dirs := SkillsDirs()
	require.Len(t, dirs, 2)

	// Verify both plugin names in installed list
	names := map[string]bool{}
	for _, p := range installed {
		names[p.Manifest.Name] = true
	}
	require.True(t, names["superpowers"])
	require.True(t, names["caveman"])
}
