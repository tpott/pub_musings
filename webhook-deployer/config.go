package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds all site configurations
type Config struct {
	Sites []SiteConfig `yaml:"sites"`
}

// SiteConfig defines a deployment target
type SiteConfig struct {
	Name         string            `yaml:"name"`
	Path         string            `yaml:"path"`          // Working directory
	PathPrefix   string            `yaml:"path_prefix"`   // Only deploy if files here changed
	Branch       string            `yaml:"branch"`
	Repository   string            `yaml:"repository"`    // e.g., "tpott/pub_musings"
	DeployScript string            `yaml:"deploy_script"` // External script path
	Commands     []string          `yaml:"commands"`      // Or inline commands
	Environment  map[string]string `yaml:"environment"`
}

// LoadConfig reads and parses the YAML configuration file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	// Validate config
	for i, site := range config.Sites {
		if site.Name == "" {
			return nil, fmt.Errorf("site %d: name is required", i)
		}
		if site.Path == "" {
			return nil, fmt.Errorf("site %q: path is required", site.Name)
		}
		if site.Branch == "" {
			return nil, fmt.Errorf("site %q: branch is required", site.Name)
		}
		if site.Repository == "" {
			return nil, fmt.Errorf("site %q: repository is required", site.Name)
		}
		if site.DeployScript == "" && len(site.Commands) == 0 {
			return nil, fmt.Errorf("site %q: either deploy_script or commands is required", site.Name)
		}
	}

	return &config, nil
}

// FindMatchingSites returns all sites that match the given branch, repository, and changed files
func (c *Config) FindMatchingSites(branch, repository string, changedFiles []string) []SiteConfig {
	var matches []SiteConfig

	for _, site := range c.Sites {
		// Check branch and repository
		if site.Branch != branch || site.Repository != repository {
			continue
		}

		// If no path_prefix specified, match any file change
		if site.PathPrefix == "" {
			matches = append(matches, site)
			continue
		}

		// Check if any changed file is under the path_prefix
		for _, file := range changedFiles {
			if strings.HasPrefix(file, site.PathPrefix) {
				matches = append(matches, site)
				break
			}
		}
	}

	return matches
}
