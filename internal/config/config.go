package config

import (
	"fmt"
	"os"
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
	Name      string       `yaml:"name"`
	Type      ScheduleType `yaml:"type"`
	Users     []string     `yaml:"users"`
	StartTime string       `yaml:"start_time"`
}

type ScheduleType string

const (
	ScheduleTypeDaily  ScheduleType = "daily"
	ScheduleTypeWeekly ScheduleType = "weekly"
)

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
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("configuration file not found at path: %s", path)
		}
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var config MainConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse yaml file %s: %w", path, err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return &config, nil
}

func (c *MainConfig) Validate() error {
	// user validate
	userIDs, err := validateUniqueness(c.Users, func(u User) string { return u.ID }, "user")
	if err != nil {
		return err
	}
	for _, user := range c.Users {
		if user.Name == "" {
			return fmt.Errorf("user with id %q is missing a name", user.ID)
		}
	}

	// schedule valudate
	scheduleNames, err := validateUniqueness(c.Schedules, func(s Schedule) string { return s.Name }, "schedule")
	if err != nil {
		return err
	}

	validEscalationTargets := make(map[string]struct{})
	for id := range userIDs {
		validEscalationTargets[id] = struct{}{}
	}
	for name := range scheduleNames {
		validEscalationTargets[name] = struct{}{}
	}

	for _, schedule := range c.Schedules {
		if err := schedule.validate(userIDs); err != nil {
			return fmt.Errorf("schedule %q is invalid: %w", schedule.Name, err)
		}
	}

	// policy validate
	for _, policy := range c.EscalationPolicies {
		if policy.Name == "" {
			return fmt.Errorf("escalation policy is missing a name")
		}

		if err := policy.validate(validEscalationTargets); err != nil {
			return fmt.Errorf("escalation policy %q is invalid: %w", policy.Name, err)
		}
	}

	return nil
}

func (s *Schedule) validate(userIDs map[string]struct{}) error {
	switch s.Type {
	case ScheduleTypeDaily, ScheduleTypeWeekly:
	case "":
		return fmt.Errorf("type is not specified")
	default:
		return fmt.Errorf("unknown type: %q", s.Type)
	}

	if _, err := time.Parse("15:04", s.StartTime); err != nil {
		return fmt.Errorf("invalid start_time format %q: expected HH:MM", s.StartTime)
	}

	if len(s.Users) == 0 {
		return fmt.Errorf("must contain at least one user")
	}
	for _, userID := range s.Users {
		if _, exists := userIDs[userID]; !exists {
			return fmt.Errorf("refers to a non-existent user ID: %s", userID)
		}
	}

	return nil
}

func (p *EscalationPolicy) validate(validTargets map[string]struct{}) error {
	if len(p.Steps) == 0 {
		return fmt.Errorf("must contain at least one escalation step")
	}

	for i, step := range p.Steps {
		if err := step.validate(validTargets); err != nil {
			return fmt.Errorf("step %d is invalid: %w", i+1, err)
		}
	}
	return nil
}

func (es *EscalationStep) validate(validTargets map[string]struct{}) error {
	if es.Target == "" {
		return fmt.Errorf("target is not specified")
	}
	if _, exists := validTargets[es.Target]; !exists {
		return fmt.Errorf("target %q does not point to a valid user or schedule", es.Target)
	}
	if es.Wait <= 0 {
		return fmt.Errorf("wait duration must be positive, but got %v", es.Wait)
	}
	return nil
}

// Helper to check for duplicates and required fields.
// It takes a slice of items, a function to get the ID/Name, and the name of the entity for error messages.
func validateUniqueness[T any](items []T, getID func(T) string, entityName string) (map[string]struct{}, error) {
	ids := make(map[string]struct{}, len(items))
	for _, item := range items {
		id := getID(item)
		if id == "" {
			return nil, fmt.Errorf("%s is missing an ID/name", entityName)
		}
		if _, exists := ids[id]; exists {
			return nil, fmt.Errorf("duplicate %s found: %s", entityName, id)
		}
		ids[id] = struct{}{}
	}
	return ids, nil
}
