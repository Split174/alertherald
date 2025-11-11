package server

import (
	"context"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/Split174/alert-herald/internal/engine"
	"github.com/Split174/alert-herald/internal/store"
	"github.com/Split174/alert-herald/internal/types"
)

type Server struct {
	httpServer *http.Server
	store      *store.Store
	engine     *engine.Engine
	templates  *template.Template
}

func NewServer(addr string, store *store.Store, engine *engine.Engine) *Server {
	tpl, err := template.ParseFiles("templates/alerts.html")
	if err != nil {
		slog.Error("Failed to parse templates", "error", err)
		panic(err)
	}

	s := &Server{
		store:     store,
		engine:    engine,
		templates: tpl,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.redirectToAlerts)
	mux.HandleFunc("GET /alerts", s.handleListAlerts)
	mux.HandleFunc("POST /api/v1/webhooks/alertmanager", s.handleWebhook)

	mux.HandleFunc("POST /alerts/{id}/{action}", s.handleAlertAction)

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return s
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) redirectToAlerts(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/alerts", http.StatusFound)
}

func (s *Server) handleListAlerts(w http.ResponseWriter, r *http.Request) {
	dbAlerts, err := s.store.GetActiveAlerts()
	if err != nil {
		slog.Error("Failed to get active alerts", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	templateAlerts := make([]types.AlertForTemplate, len(dbAlerts))
	for i, a := range dbAlerts {
		templateAlerts[i] = types.AlertForTemplate{
			Alert:          a,
			LabelsMap:      a.GetLabelsMap(),
			AnnotationsMap: a.GetAnnotationsMap(),
		}
	}

	data := struct {
		Alerts []types.AlertForTemplate
	}{
		Alerts: templateAlerts,
	}

	err = s.templates.ExecuteTemplate(w, "alerts.html", data)
	if err != nil {
		slog.Error("Failed to execute template", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var webhook types.AlertmanagerWebhook
	if err := json.NewDecoder(r.Body).Decode(&webhook); err != nil {
		slog.Error("Failed to decode alertmanager webhook", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	s.engine.ProcessWebhook(webhook)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (s *Server) handleAlertAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 3 {
		http.NotFound(w, r)
		return
	}

	alertIDStr := parts[1]
	action := parts[2]

	alertID, err := strconv.ParseInt(alertIDStr, 10, 64)
	if err != nil {
		slog.Error("Invalid alert ID", "id", alertIDStr)
		http.Error(w, "Bad Request: Invalid ID", http.StatusBadRequest)
		return
	}

	user := "web-ui"

	switch action {
	case "ack":
		err = s.store.AcknowledgeAlert(alertID, user)
		slog.Info("Alert acknowledged via UI", "alert_id", alertID)
	case "resolve":
		err = s.store.ResolveAlert(alertID, user)
		slog.Info("Alert resolved via UI", "alert_id", alertID)
	default:
		http.NotFound(w, r)
		return
	}

	if err != nil {
		slog.Error("Failed to perform alert action", "action", action, "alert_id", alertID, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/alerts", http.StatusFound)
}
