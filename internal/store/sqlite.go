package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Split174/alert-herald/internal/types"
	_ "github.com/mattn/go-sqlite3"
)

const schema = `
CREATE TABLE IF NOT EXISTS alerts (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    fingerprint   TEXT NOT NULL UNIQUE,
    status        TEXT NOT NULL,
    labels        TEXT,
    annotations   TEXT,
    starts_at     DATETIME,
	ends_at		  DATETIME,
    last_notified DATETIME,
    assigned_to   TEXT,
    current_step  INTEGER DEFAULT 0,
    created_at    DATETIME NOT NULL,
    updated_at    DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_alerts_status ON alerts(status);

CREATE TABLE IF NOT EXISTS alert_log (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    alert_id   INTEGER NOT NULL,
    timestamp  DATETIME NOT NULL,
    message    TEXT NOT NULL,
    FOREIGN KEY(alert_id) REFERENCES alerts(id) ON DELETE CASCADE
);
`

type Store struct {
	db *sql.DB
}

func NewStore(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to apply schema: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) GetActiveAlerts() ([]types.Alert, error) {
	rows, err := s.db.Query(`SELECT id, fingerprint, status, labels, annotations, starts_at, assigned_to, current_step, updated_at FROM alerts WHERE status = 'firing' OR status = 'acknowledged' ORDER BY starts_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []types.Alert
	for rows.Next() {
		var a types.Alert
		err := rows.Scan(&a.ID, &a.Fingerprint, &a.Status, &a.Labels, &a.Annotations, &a.StartsAt, &a.AssignedTo, &a.CurrentStep, &a.UpdatedAt)
		if err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}
	return alerts, nil
}

func (s *Store) FindOrCreateAlert(amAlert types.AlertmanagerAlert) (types.Alert, bool, error) {
	var alert types.Alert
	var created bool

	tx, err := s.db.Begin()
	if err != nil {
		return alert, false, err
	}
	defer tx.Rollback()

	row := tx.QueryRow(`SELECT id, fingerprint, status FROM alerts WHERE fingerprint = ?`, amAlert.Fingerprint)
	err = row.Scan(&alert.ID, &alert.Fingerprint, &alert.Status)

	labelsJSON, _ := json.Marshal(amAlert.Labels)
	annotationsJSON, _ := json.Marshal(amAlert.Annotations)

	now := time.Now().UTC()

	if err == sql.ErrNoRows {
		// create new alert
		res, err := tx.Exec(
			`INSERT INTO alerts (fingerprint, status, labels, annotations, starts_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			amAlert.Fingerprint, amAlert.Status, string(labelsJSON), string(annotationsJSON), amAlert.StartsAt.UTC(), now, now,
		)
		if err != nil {
			return alert, false, err
		}
		id, _ := res.LastInsertId()
		alert.ID = id
		alert.Status = amAlert.Status
		created = true
		_ = s.LogAction(tx, alert.ID, fmt.Sprintf("Alert created with status: %s", alert.Status))
	} else if err != nil {
		return alert, false, err
	} else {
		// update status
		if alert.Status != amAlert.Status {
			_, err := tx.Exec(
				`UPDATE alerts SET status = ?, ends_at = ?, updated_at = ? WHERE id = ?`,
				amAlert.Status, amAlert.EndsAt.UTC(), now, alert.ID,
			)
			if err != nil {
				return alert, false, err
			}
			alert.Status = amAlert.Status
			_ = s.LogAction(tx, alert.ID, fmt.Sprintf("Alert status changed to: %s", alert.Status))
		}
	}

	return alert, created, tx.Commit()
}

func (s *Store) UpdateAlertEscalation(alertID int64, step int, assignedTo string) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(`UPDATE alerts SET current_step = ?, assigned_to = ?, last_notified = ?, updated_at = ? WHERE id = ?`,
		step, assignedTo, now, now, alertID)

	if err == nil {
		_ = s.LogAction(nil, alertID, fmt.Sprintf("Escalated to step %d, assigned to %s", step, assignedTo))
	}
	return err
}

func (s *Store) AcknowledgeAlert(alertID int64, user string) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(`UPDATE alerts SET status = 'acknowledged', updated_at = ? WHERE id = ? AND status = 'firing'`,
		now, alertID)

	if err == nil {
		_ = s.LogAction(nil, alertID, fmt.Sprintf("Alert acknowledged by %s", user))
	}
	return err
}

func (s *Store) ResolveAlert(alertID int64, user string) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(`UPDATE alerts SET status = ?, updated_at = ?, ends_at = ? WHERE id = ?`,
		"resolved", now, now, alertID)

	if err == nil {
		_ = s.LogAction(nil, alertID, fmt.Sprintf("Alert manually resolved by %s", user))
	}
	return err
}

func (s *Store) GetAlertByID(id int64) (*types.Alert, error) {
	row := s.db.QueryRow(`SELECT id, fingerprint, status, labels, annotations, starts_at, assigned_to, current_step FROM alerts WHERE id = ?`, id)
	var a types.Alert
	err := row.Scan(&a.ID, &a.Fingerprint, &a.Status, &a.Labels, &a.Annotations, &a.StartsAt, &a.AssignedTo, &a.CurrentStep)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Store) LogAction(tx *sql.Tx, alertID int64, message string) error {
	now := time.Now().UTC()
	var err error
	if tx != nil {
		_, err = tx.Exec(`INSERT INTO alert_log (alert_id, timestamp, message) VALUES (?, ?, ?)`, alertID, now, message)
	} else {
		_, err = s.db.Exec(`INSERT INTO alert_log (alert_id, timestamp, message) VALUES (?, ?, ?)`, alertID, now, message)
	}
	return err
}
