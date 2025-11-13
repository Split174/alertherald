package config

import (
	"reflect"
	"testing"
	"time"
)

func TestValidateSchedule(t *testing.T) {
	existingUserIDs := map[string]struct{}{
		"user-1": {},
		"user-2": {},
	}

	testCases := []struct {
		name     string
		schedule Schedule
		userIDs  map[string]struct{}
		wantErr  bool
	}{
		{
			name: "Valid daily schedule",
			schedule: Schedule{
				Name:      "developers",
				Type:      "daily",
				Users:     []string{"user-1", "user-2"},
				StartTime: "10:00",
			},
			userIDs: existingUserIDs,
			wantErr: false,
		},
		{
			name: "Valid weekly schedule",
			schedule: Schedule{
				Name:      "developers",
				Type:      "weekly",
				Users:     []string{"user-1", "user-2"},
				StartTime: "10:00",
			},
			userIDs: existingUserIDs,
			wantErr: false,
		},
		{
			name: "Unknown schedule type",
			schedule: Schedule{
				Name:      "developers",
				Type:      "month",
				Users:     []string{"user-1", "user-2"},
				StartTime: "10:00",
			},
			userIDs: existingUserIDs,
			wantErr: true,
		},
		{
			name: "Empy schedule type",
			schedule: Schedule{
				Name:      "developers",
				Type:      "",
				Users:     []string{"user-1", "user-2"},
				StartTime: "10:00",
			},
			userIDs: existingUserIDs,
			wantErr: true,
		},
		{
			name: "Invalid time",
			schedule: Schedule{
				Name:      "developers",
				Type:      "weekly",
				Users:     []string{"user-1", "user-2"},
				StartTime: "10:00:05",
			},
			userIDs: existingUserIDs,
			wantErr: true,
		},
		{
			name: "No user for schedule",
			schedule: Schedule{
				Name:      "developers",
				Type:      "weekly",
				Users:     []string{},
				StartTime: "10:00:05",
			},
			userIDs: existingUserIDs,
			wantErr: true,
		},
		{
			name: "No exist user",
			schedule: Schedule{
				Name:      "developers",
				Type:      "weekly",
				Users:     []string{"user-1", "ghost"},
				StartTime: "10:00:05",
			},
			userIDs: existingUserIDs,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.schedule.validate(tc.userIDs)
			if (err != nil) != tc.wantErr {
				t.Errorf("Schedule.validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}

}

func TestValidatePolicy(t *testing.T) {

	validEscalationTargets := map[string]struct{}{
		"user-1":           {},
		"user-2":           {},
		"daily-developers": {},
	}

	testCases := []struct {
		name              string
		policy            EscalationPolicy
		escalationTargets map[string]struct{}
		wantErr           bool
	}{
		{
			name: "Valid policy",
			policy: EscalationPolicy{
				Name: "database",
				Match: Labels{
					"severity": "critical",
				},
				Steps: []EscalationStep{
					{
						Target: "user-1",
						Wait:   30 * time.Minute,
					},
				},
			},
			escalationTargets: validEscalationTargets,
			wantErr:           false,
		},
		// TODO
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.policy.validate(tc.escalationTargets)
			if (err != nil) != tc.wantErr {
				t.Errorf("Schedule.validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}

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
