package appconfig

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	defaultRegistryOwner = "siiway"
	defaultRegistryRepo  = "cli-templates"
	defaultRegistryRef   = "main"
	defaultRegistryPath  = "templates.yaml"
	defaultAlias         = "default"
)

// Config is the user-level CLI configuration persisted on disk.
type Config struct {
	CurrentRegistry string                    `json:"current_registry" yaml:"current_registry"`
	Registries      map[string]RegistryConfig `json:"registries" yaml:"registries"`
	GitHubToken     string                    `json:"github_token" yaml:"github_token"`
	Languages       map[string]LanguageConfig `json:"languages" yaml:"languages"`
}

// LanguageConfig defines language-specific command settings.
type LanguageConfig struct {
	Run     map[string]string     `json:"run" yaml:"run"`
	Version LanguageVersionConfig `json:"version" yaml:"version"`
}

// LanguageVersionConfig defines runtime version backend/template settings per language.
type LanguageVersionConfig struct {
	Backend string `json:"backend" yaml:"backend"`
	Use     string `json:"use" yaml:"use"`
}

// RegistryConfig points to a GitHub repository that contains templates metadata.
type RegistryConfig struct {
	Owner string `json:"owner" yaml:"owner"`
	Repo  string `json:"repo" yaml:"repo"`
	Ref   string `json:"ref" yaml:"ref"`
	Path  string `json:"path" yaml:"path"`
}

// Default returns the default CLI configuration.
func Default() Config {
	return Config{
		CurrentRegistry: defaultAlias,
		Registries: map[string]RegistryConfig{
			defaultAlias: defaultRegistryConfig(),
		},
	}
}

func defaultRegistryConfig() RegistryConfig {
	return RegistryConfig{
		Owner: defaultRegistryOwner,
		Repo:  defaultRegistryRepo,
		Ref:   defaultRegistryRef,
		Path:  defaultRegistryPath,
	}
}

func configFilePath() (string, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfgDir, "siiway", "config.yaml"), nil
}

// Path returns the absolute path to the user config file.
func Path() (string, error) {
	return configFilePath()
}

// Load reads configuration from disk, falling back to defaults when missing.
func Load() (Config, error) {
	cfg := Default()
	p, err := configFilePath()
	if err != nil {
		return cfg, err
	}

	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Default(), err
	}

	cfg.applyDefaults()
	return cfg, nil
}

// Save persists configuration to disk.
func Save(cfg Config) error {
	cfg.applyDefaults()

	p, err := configFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(p, append(data, '\n'), 0o644)
}

func (c *Config) applyDefaults() {
	if c.Registries == nil {
		c.Registries = map[string]RegistryConfig{}
	}

	if len(c.Registries) == 0 {
		c.Registries[defaultAlias] = defaultRegistryConfig()
	}

	for alias, reg := range c.Registries {
		normalizedAlias := strings.TrimSpace(alias)
		if normalizedAlias == "" {
			continue
		}

		reg.Owner = strings.TrimSpace(reg.Owner)
		reg.Repo = strings.TrimSpace(reg.Repo)
		reg.Ref = strings.TrimSpace(reg.Ref)
		reg.Path = strings.Trim(strings.TrimSpace(reg.Path), "/")

		if reg.Owner == "" {
			reg.Owner = defaultRegistryOwner
		}
		if reg.Repo == "" {
			reg.Repo = defaultRegistryRepo
		}
		if reg.Ref == "" {
			reg.Ref = defaultRegistryRef
		}
		if reg.Path == "" {
			reg.Path = defaultRegistryPath
		}

		if normalizedAlias != alias {
			delete(c.Registries, alias)
		}
		c.Registries[normalizedAlias] = reg
	}

	c.CurrentRegistry = strings.TrimSpace(c.CurrentRegistry)
	if c.CurrentRegistry == "" {
		c.CurrentRegistry = defaultAlias
	}
	if _, ok := c.Registries[c.CurrentRegistry]; !ok {
		c.CurrentRegistry = defaultAlias
	}
	if _, ok := c.Registries[defaultAlias]; !ok {
		c.Registries[defaultAlias] = defaultRegistryConfig()
	}

	c.GitHubToken = strings.TrimSpace(c.GitHubToken)

	if c.Languages == nil {
		c.Languages = map[string]LanguageConfig{}
	}
	for lang, langCfg := range c.Languages {
		normalizedLang := strings.ToLower(strings.TrimSpace(lang))
		if normalizedLang == "" {
			continue
		}

		normalizedRun := map[string]string{}
		for action, command := range langCfg.Run {
			normalizedAction := strings.ToLower(strings.TrimSpace(action))
			normalizedCommand := strings.TrimSpace(command)
			if normalizedAction == "" || normalizedCommand == "" {
				continue
			}
			normalizedRun[normalizedAction] = normalizedCommand
		}
		langCfg.Run = normalizedRun
		langCfg.Version.Backend = strings.ToLower(strings.TrimSpace(langCfg.Version.Backend))
		langCfg.Version.Use = strings.TrimSpace(langCfg.Version.Use)

		if normalizedLang != lang {
			delete(c.Languages, lang)
		}
		c.Languages[normalizedLang] = langCfg
	}
}

// ActiveRegistry returns the currently selected registry configuration.
func (c Config) ActiveRegistry() RegistryConfig {
	if reg, ok := c.Registries[c.CurrentRegistry]; ok {
		return reg
	}
	return defaultRegistryConfig()
}
