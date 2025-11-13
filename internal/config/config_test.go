package config

import (
	"reflect"
	"testing"
	"time"
)

func TestValidateSchedule(t *testing.T) {

}

func TestParseConfig(t *testing.T) {
	actualConfig, err := Load("./testconfig.yaml")
	if actualConfig == nil || err != nil {
		t.Fatalf("Error parse config %q", err.Error())
	}

	expectedConfig := MainConfig{
		Users: []User{
			{
				ID:         "user-1",
				Name:       "Alice",
				TelegramID: 123456789,
			},
			{
				ID:         "user-2",
				Name:       "Bob",
				TelegramID: 987654321,
			},
			{
				ID:         "user-3-ceo",
				Name:       "Charlie",
				TelegramID: 555555555,
			},
		},
		Schedules: []Schedule{
			{
				Name:      "daily-dev-rotation",
				Type:      "daily",
				Users:     []string{"user-1", "user-2"},
				StartTime: "10:00",
			},
		},
		EscalationPolicies: []EscalationPolicy{
			{
				Name: "Critical DB Policy",
				Match: map[string]string{
					"severity": "critical",
					"service":  "database",
				},
				Steps: []EscalationStep{
					{
						Target: "daily-dev-rotation",
						Wait:   5 * time.Minute,
					},
					{
						Target: "user-3-ceo",
						Wait:   15 * time.Minute,
					},
					{
						Target: "user-3-ceo",
						Wait:   30 * time.Minute,
					},
				},
			},
			{
				Name: "Default Warning Policy",
				Match: map[string]string{
					"severity": "warning",
				},
				Steps: []EscalationStep{
					{
						Target: "user-1",
						Wait:   1 * time.Hour,
					},
				},
			},
		},
	}

	if !reflect.DeepEqual(expectedConfig, *actualConfig) {
		t.Errorf(`Expected config does not match actual config
actual: %+v
expected: %+v
		`, actualConfig, expectedConfig)
	}
}
