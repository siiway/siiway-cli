package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/SiiWay/siiway-cli/internal/appconfig"
	"github.com/SiiWay/siiway-cli/internal/registry"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configResetCmd)
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage siiway CLI configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := appconfig.Load()
		if err != nil {
			return fmt.Errorf("failed loading config: %w", err)
		}
		cfgPath, err := appconfig.Path()
		if err != nil {
			return fmt.Errorf("failed resolving config path: %w", err)
		}

		fmt.Printf("Config file: %s\n", cfgPath)
		fmt.Printf("Current registry alias: %s\n", cfg.CurrentRegistry)
		fmt.Printf("Default GitHub token: %s\n", maskToken(cfg.GitHubToken))

		aliases := make([]string, 0, len(cfg.Registries))
		for alias := range cfg.Registries {
			aliases = append(aliases, alias)
		}
		sort.Strings(aliases)

		for _, alias := range aliases {
			reg := cfg.Registries[alias]
			source := registry.Source{
				Owner: reg.Owner,
				Repo:  reg.Repo,
				Ref:   reg.Ref,
				Path:  reg.Path,
			}
			mark := " "
			if alias == cfg.CurrentRegistry {
				mark = "*"
			}

			fmt.Printf("\n[%s] %s\n", mark, alias)
			fmt.Printf("  owner: %s\n", reg.Owner)
			fmt.Printf("  repo: %s\n", reg.Repo)
			fmt.Printf("  ref: %s\n", reg.Ref)
			fmt.Printf("  path: %s\n", reg.Path)
			fmt.Printf("  url: %s\n", registry.SourcePathFor(source))
		}
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key_name> <value>",
	Short: "Set a config value by key",
	Long: "Supported keys:\n" +
		"  token (alias of github_token)\n" +
		"  github_token\n" +
		"  current_registry\n" +
		"  registries.<alias>.owner\n" +
		"  registries.<alias>.repo\n" +
		"  registries.<alias>.ref\n" +
		"  registries.<alias>.path\n" +
		"  registries.<alias>.source   (value format: owner/repo)",
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := strings.TrimSpace(args[0])
		value := strings.TrimSpace(args[1])
		if key == "" {
			return fmt.Errorf("key_name cannot be empty")
		}

		cfg, err := appconfig.Load()
		if err != nil {
			return fmt.Errorf("failed loading config: %w", err)
		}

		if err := applyConfigSetKey(&cfg, key, value); err != nil {
			return err
		}

		if err := appconfig.Save(cfg); err != nil {
			return fmt.Errorf("failed saving config: %w", err)
		}

		fmt.Printf("Config updated: %s\n", key)
		return nil
	},
}

var configResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset configuration to defaults",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := appconfig.Default()
		if err := appconfig.Save(cfg); err != nil {
			return fmt.Errorf("failed resetting config: %w", err)
		}
		fmt.Println("Configuration reset to defaults")
		return nil
	},
}

func maskToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return "(not set)"
	}
	if len(token) <= 8 {
		return "********"
	}
	return token[:4] + strings.Repeat("*", len(token)-8) + token[len(token)-4:]
}

func applyConfigSetKey(cfg *appconfig.Config, key, value string) error {
	if value == "" {
		return fmt.Errorf("value cannot be empty")
	}

	if cfg.Registries == nil {
		cfg.Registries = map[string]appconfig.RegistryConfig{}
	}

	switch key {
	case "token", "github_token":
		cfg.GitHubToken = value
		return nil
	case "current_registry":
		if _, ok := cfg.Registries[value]; !ok {
			return fmt.Errorf("registry alias not found: %s", value)
		}
		cfg.CurrentRegistry = value
		return nil
	}

	parts := strings.Split(key, ".")
	if len(parts) != 3 || parts[0] != "registries" {
		return fmt.Errorf("unsupported key: %s", key)
	}

	alias := strings.TrimSpace(parts[1])
	field := strings.TrimSpace(parts[2])
	if alias == "" || field == "" {
		return fmt.Errorf("invalid key: %s", key)
	}

	reg, ok := cfg.Registries[alias]
	if !ok {
		reg = cfg.ActiveRegistry()
	}

	switch field {
	case "owner":
		reg.Owner = value
	case "repo":
		reg.Repo = value
	case "ref":
		reg.Ref = value
	case "path":
		reg.Path = strings.Trim(value, "/")
	case "source":
		sourceParts := strings.Split(value, "/")
		if len(sourceParts) != 2 || strings.TrimSpace(sourceParts[0]) == "" || strings.TrimSpace(sourceParts[1]) == "" {
			return fmt.Errorf("invalid source value, expected owner/repo: %s", value)
		}
		reg.Owner = strings.TrimSpace(sourceParts[0])
		reg.Repo = strings.TrimSpace(sourceParts[1])
	default:
		return fmt.Errorf("unsupported key field: %s", field)
	}

	cfg.Registries[alias] = reg
	return nil
}
