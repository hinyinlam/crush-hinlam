package cmd

import (
	"fmt"

	"github.com/charmbracelet/crush/internal/plugins"
	"github.com/spf13/cobra"
)

var pluginCmd = &cobra.Command{
	Use:   "plugin [command]",
	Short: "Manage Claude Code plugins",
	Long: `Install and manage Claude Code plugins (e.g. superpowers, caveman).

Plugins are git repositories containing a .claude-plugin/plugin.json manifest
and optionally a skills/ directory with SKILL.md files. Installed plugins'
skills are automatically discovered by Crush.`,
}

var pluginInstallCmd = &cobra.Command{
	Use:   "install <repo>",
	Short: "Install a Claude Code plugin from a git repository",
	Long: `Clone a plugin repository and register its skills.

Examples:
  crush plugin install obra/superpowers
  crush plugin install https://github.com/obra/superpowers
  crush plugin install JuliusBrussee/caveman`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo := args[0]
		fmt.Printf("Installing plugin from %s...\n", repo)

		result, err := plugins.Install(repo)
		if err != nil {
			return fmt.Errorf("failed to install plugin: %w", err)
		}

		fmt.Printf("✓ Installed: %s\n", result.Manifest.Name)
		if result.Manifest.Description != "" {
			fmt.Printf("  Description: %s\n", result.Manifest.Description)
		}
		if result.Manifest.Version != "" {
			fmt.Printf("  Version: %s\n", result.Manifest.Version)
		}
		fmt.Printf("  Path: %s\n", result.Path)

		// Check for skills directory
		skillsFound := false
		if dirs := plugins.SkillsDirs(); len(dirs) > 0 {
			skillsFound = true
		}
		if skillsFound {
			fmt.Printf("  Skills will be available on next Crush startup.\n")
		}

		return nil
	},
}

var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed Claude Code plugins",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		installed, err := plugins.ListInstalled()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		if len(installed) == 0 {
			fmt.Println("No plugins installed.")
			fmt.Println("Install one with: crush plugin install <owner/repo>")
			return
		}
		for _, p := range installed {
			fmt.Printf("  %s", p.Manifest.Name)
			if p.Manifest.Version != "" {
				fmt.Printf(" v%s", p.Manifest.Version)
			}
			if p.Manifest.Description != "" {
				fmt.Printf(" — %s", p.Manifest.Description)
			}
			fmt.Println()
		}
	},
}

func init() {
	pluginCmd.AddCommand(pluginInstallCmd, pluginListCmd)
}
