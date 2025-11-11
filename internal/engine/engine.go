package engine

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/Split174/alert-herald/internal/config"
	"github.com/Split174/alert-herald/internal/notification"
	"github.com/Split174/alert-herald/internal/store"
	"github.com/Split174/alert-herald/internal/types"
)

// lifecycle alert and escalation
type Engine struct {
	store    *store.Store
	notifier notification.Notifier

	configMtx sync.RWMutex
	config    *config.MainConfig

	activeEscalations sync.Map
}

func NewEngine(store *store.Store, notifier notification.Notifier, cfg *config.MainConfig) (*Engine, error) {
	return &Engine{
		store:    store,
		notifier: notifier,
		config:   cfg,
	}, nil
}

func (e *Engine) UpdateConfig(newCfg *config.MainConfig) {
	e.configMtx.Lock()
	defer e.configMtx.Unlock()
	e.config = newCfg
}

func (e *Engine) Stop() {
	slog.Info("Stopping engine, cancelling all active escalations...")
	e.activeEscalations.Range(func(key, value interface{}) bool {
		cancel := value.(context.CancelFunc)
		cancel()
		slog.Info("Cancelled escalation", "fingerprint", key)
		return true
	})
}

func (e *Engine) ProcessWebhook(payload types.AlertmanagerWebhook) {
	for _, amAlert := range payload.Alerts {
		go e.processSingleAlert(amAlert)
	}
}

func (e *Engine) processSingleAlert(amAlert types.AlertmanagerAlert) {
	slog.Info("Processing alert", "fingerprint", amAlert.Fingerprint, "status", amAlert.Status)

	dbAlert, created, err := e.store.FindOrCreateAlert(amAlert)
	if err != nil {
		slog.Error("Failed to find or create alert in DB", "fingerprint", amAlert.Fingerprint, "error", err)
		return
	}

	if amAlert.Status == "resolved" {
		if cancelFunc, ok := e.activeEscalations.Load(dbAlert.Fingerprint); ok {
			cancelFunc.(context.CancelFunc)()
			e.activeEscalations.Delete(dbAlert.Fingerprint)
			slog.Info("Stopped escalation for resolved alert", "fingerprint", dbAlert.Fingerprint)
		}
		return
	}

	if amAlert.Status == "firing" && created {
		policy := e.findPolicyForAlert(amAlert.Labels)
		if policy == nil {
			slog.Warn("No escalation policy found for alert", "fingerprint", amAlert.Fingerprint, "labels", amAlert.Labels)
			return
		}
		slog.Info("Found matching policy", "alert_name", amAlert.Labels["alertname"], "policy_name", policy.Name)

		ctx, cancelFunc := context.WithCancel(context.Background())
		e.activeEscalations.Store(dbAlert.Fingerprint, cancelFunc)

		go e.runEscalation(ctx, dbAlert, *policy)
	}
}

func (e *Engine) findPolicyForAlert(alertLabels map[string]string) *config.EscalationPolicy {
	e.configMtx.RLock()
	defer e.configMtx.RUnlock()

	for _, policy := range e.config.EscalationPolicies {
		if policy.Match == nil {
			continue
		}
		matches := true
		for k, v := range policy.Match {
			if alertVal, ok := alertLabels[k]; !ok || alertVal != v {
				matches = false
				break
			}
		}
		if matches {
			return &policy
		}
	}
	return nil
}

func (e *Engine) runEscalation(ctx context.Context, alert types.Alert, policy config.EscalationPolicy) {
	defer func() {
		if cancelFunc, ok := e.activeEscalations.Load(alert.Fingerprint); ok {
			cancelFunc.(context.CancelFunc)()
			e.activeEscalations.Delete(alert.Fingerprint)
		}
	}()

	for i, step := range policy.Steps {
		slog.Info("Executing escalation step", "fingerprint", alert.Fingerprint, "step", i)

		user := e.getTargetUser(step.Target)
		if user == nil {
			slog.Error("Could not find target user for step", "target", step.Target, "step", i)
			continue
		}

		err := e.store.UpdateAlertEscalation(alert.ID, i, user.Name)
		if err != nil {
			slog.Error("Failed to update alert escalation state in DB", "error", err)
			return
		}

		message := e.formatNotificationMessage(alert, policy)
		err = e.notifier.Notify(user.TelegramID, message)
		if err != nil {
			slog.Error("Failed to send notification", "user", user.Name, "error", err)
		} else {
			slog.Info("Notification sent", "user", user.Name, "fingerprint", alert.Fingerprint)
		}

		select {
		case <-time.After(step.Wait):
			currentAlert, err := e.store.GetAlertByID(alert.ID)
			if err != nil {
				slog.Error("Could not get alert status before next step", "error", err)
				return
			}
			if currentAlert.Status == "acknowledged" || currentAlert.Status == "resolved" {
				slog.Info("Escalation halted, alert was acknowledged or resolved", "fingerprint", alert.Fingerprint)
				return
			}
			continue
		case <-ctx.Done():
			slog.Info("Escalation context cancelled", "fingerprint", alert.Fingerprint)
			return
		}
	}
	slog.Info("Escalation policy finished for alert", "fingerprint", alert.Fingerprint)
}

func (e *Engine) getTargetUser(target string) *config.User {
	e.configMtx.RLock()
	defer e.configMtx.RUnlock()

	for _, u := range e.config.Users {
		if u.ID == target {
			return &u
		}
	}

	for _, s := range e.config.Schedules {
		if s.Name == target {
			if s.Type == "daily" && len(s.Users) > 0 {
				now := time.Now()

				parts := strings.Split(s.StartTime, ":")
				if len(parts) != 2 {
					slog.Error("Invalid start_time format in schedule", "schedule", s.Name)
					return nil
				}

				var h, m int
				fmt.Sscanf(s.StartTime, "%d:%d", &h, &m)

				epoch := time.Date(1970, 1, 1, h, m, 0, 0, time.UTC)

				daysSinceEpoch := int(now.Sub(epoch).Hours() / 24)

				onCallIndex := daysSinceEpoch % len(s.Users)
				if onCallIndex < 0 {
					onCallIndex += len(s.Users)
				}

				onCallUserID := s.Users[onCallIndex]

				return e.getTargetUser(onCallUserID)
			}
		}
	}

	return nil
}

// formatNotificationMessage
func (e *Engine) formatNotificationMessage(alert types.Alert, policy config.EscalationPolicy) string {
	var builder strings.Builder
	labels := alert.GetLabelsMap()
	annotations := alert.GetAnnotationsMap()

	builder.WriteString(fmt.Sprintf("*FIRING*\n\n"))

	if name, ok := labels["alertname"]; ok {
		builder.WriteString(fmt.Sprintf("*Alert*: `%s`\n", escapeMarkdown(name)))
	}
	if severity, ok := labels["severity"]; ok {
		builder.WriteString(fmt.Sprintf("*Severity*: `%s`\n", escapeMarkdown(severity)))
	}
	if summary, ok := annotations["summary"]; ok {
		builder.WriteString(fmt.Sprintf("*Summary*: %s\n", escapeMarkdown(summary)))
	}
	if description, ok := annotations["description"]; ok {
		builder.WriteString(fmt.Sprintf("*Description*: %s\n", escapeMarkdown(description)))
	}

	builder.WriteString(fmt.Sprintf("\n*Policy*: `%s`\n", escapeMarkdown(policy.Name)))

	builder.WriteString("\n*Labels*:\n")
	for k, v := range labels {
		builder.WriteString(fmt.Sprintf(" \\- `%s`: `%s`\n", escapeMarkdown(k), escapeMarkdown(v)))
	}

	return builder.String()
}

// escapeMarkdown for Telegram MarkdownV2
func escapeMarkdown(s string) string {
	chars := []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
	for _, char := range chars {
		s = strings.ReplaceAll(s, char, "\\"+char)
	}
	return s
}
