package types

import (
	"encoding/json"
	"time"
)

type AlertmanagerAlert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
}

type AlertmanagerWebhook struct {
	Version           string              `json:"version"`
	GroupKey          string              `json:"groupKey"`
	TruncatedAlerts   int                 `json:"truncatedAlerts"`
	Status            string              `json:"status"`
	Receiver          string              `json:"receiver"`
	GroupLabels       map[string]string   `json:"groupLabels"`
	CommonLabels      map[string]string   `json:"commonLabels"`
	CommonAnnotations map[string]string   `json:"commonAnnotations"`
	ExternalURL       string              `json:"externalURL"`
	Alerts            []AlertmanagerAlert `json:"alerts"`
}

// Alert in herald
type Alert struct {
	ID           int64
	Fingerprint  string
	Status       string // "firing", "resolved", "acknowledged"
	Labels       string // JSON
	Annotations  string // JSON
	StartsAt     time.Time
	EndsAt       time.Time
	LastNotified time.Time
	AssignedTo   string // User.Name or Schedule.Name
	CurrentStep  int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type AlertLog struct {
	ID        int64
	AlertID   int64
	Timestamp time.Time
	Message   string
}

type AlertForTemplate struct {
	Alert
	LabelsMap      map[string]string
	AnnotationsMap map[string]string
}

func (a *Alert) GetLabelsMap() map[string]string {
	var labels map[string]string
	_ = json.Unmarshal([]byte(a.Labels), &labels)
	return labels
}

func (a *Alert) GetAnnotationsMap() map[string]string {
	var annotations map[string]string
	_ = json.Unmarshal([]byte(a.Annotations), &annotations)
	return annotations
}
