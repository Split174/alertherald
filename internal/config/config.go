package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type MainConfig struct {
	Users              []User             `yaml:"users"`
	Schedules          []Schedule         `yaml:"schedules"`
	EscalationPolicies []EscalationPolicy `yaml:"escalation_policies"`
}

type User struct {
	ID         string `yaml:"id"`
	Name       string `yaml:"name"`
	TelegramID int64  `yaml:"telegram_id"`
	Email      string `yaml:"email"`
	SlackID    string `yaml:"slack_id"`
}

type Schedule struct {
	Name      string   `yaml:"name"`
	Type      string   `yaml:"type"`
	Users     []string `yaml:"users"`
	StartTime string   `yaml:"start_time"`
}

type EscalationPolicy struct {
	Name  string           `yaml:"name"`
	Match Labels           `yaml:"match"`
	Steps []EscalationStep `yaml:"steps"`
}

type EscalationStep struct {
	Target string        `yaml:"target"`
	Wait   time.Duration `yaml:"wait"`
}

// Labels - for matching
type Labels map[string]string

func Load(path string) (*MainConfig, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat config path %s: %w", path, err)
	}

	var files []string
	if fi.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read config directory %s: %w", path, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() && (strings.HasSuffix(entry.Name(), ".yml") || strings.HasSuffix(entry.Name(), ".yaml")) {
				files = append(files, filepath.Join(path, entry.Name()))
			}
		}
	} else {
		files = append(files, path)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no yaml configuration files found in %s", path)
	}

	mergedConfig := &MainConfig{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file %s: %w", file, err)
		}

		var partialConfig MainConfig
		if err := yaml.Unmarshal(data, &partialConfig); err != nil {
			return nil, fmt.Errorf("failed to parse yaml file %s: %w", file, err)
		}

		mergedConfig.Users = append(mergedConfig.Users, partialConfig.Users...)
		mergedConfig.Schedules = append(mergedConfig.Schedules, partialConfig.Schedules...)
		mergedConfig.EscalationPolicies = append(mergedConfig.EscalationPolicies, partialConfig.EscalationPolicies...)
	}

	// TODO: need validation
	return mergedConfig, nil
}
