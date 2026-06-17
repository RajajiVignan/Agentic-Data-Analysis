package api

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"insightpilot/internal/store"
)

type ScheduleFrequency string

const (
	FreqDaily   ScheduleFrequency = "daily"
	FreqWeekly  ScheduleFrequency = "weekly"
	FreqMonthly ScheduleFrequency = "monthly"
)

type AlertCondition string

const (
	AlertDrop   AlertCondition = "drop"
	AlertRise   AlertCondition = "rise"
	AlertCustom AlertCondition = "custom"
)

type ScheduledReport struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	DatasetIDs   []string          `json:"datasetIds"`
	ChartIDs     []string          `json:"chartIds"`
	Frequency    ScheduleFrequency `json:"frequency"`
	DayOfWeek    int               `json:"dayOfWeek"`
	DayOfMonth   int               `json:"dayOfMonth"`
	Hour         int               `json:"hour"`
	Emails       []string          `json:"emails"`
	SlackWebhook string            `json:"slackWebhook,omitempty"`
	TeamsWebhook string            `json:"teamsWebhook,omitempty"`
	LastSent     string            `json:"lastSent,omitempty"`
	NextRun      string            `json:"nextRun,omitempty"`
	Enabled      bool              `json:"enabled"`
	CreatedAt    string            `json:"createdAt"`
}

type AlertRule struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	DatasetID   string         `json:"datasetId"`
	MetricCol   string         `json:"metricCol"`
	Condition   AlertCondition `json:"condition"`
	Threshold   float64        `json:"threshold"`
	Period      string         `json:"period"`
	Emails      []string       `json:"emails"`
	SlackHook   string         `json:"slackHook,omitempty"`
	Enabled     bool           `json:"enabled"`
	LastChecked string         `json:"lastChecked,omitempty"`
	CreatedAt   string         `json:"createdAt"`
}

const (
	maxEmailBodyBytes  = 10 * 1024   // 10 KB max email body
	maxWebhookBodyBytes = 2 * 1024   // 2 KB max webhook body
	alertCooldownMinutes = 60        // don't re-fire same alert within 60 minutes
)

type ReportService struct {
	mu            sync.RWMutex
	reports       map[string]*ScheduledReport
	alerts        map[string]*AlertRule
	alertFiredAt  map[string]time.Time // alert ID → last time notification was sent
	stopCh        chan struct{}
	smtpConfig    *SMTPConfig
	db            reportStore
	cooldownDur   time.Duration
}

type reportStore interface {
	SaveReport(r *ScheduledReport) error
	LoadReports() ([]*ScheduledReport, error)
	DeleteReport(id string) error
	SaveAlert(a *AlertRule) error
	LoadAlerts() ([]*AlertRule, error)
	DeleteAlert(id string) error
	SaveLayout(id string, layout *DashboardLayout) error
	LoadLayouts() ([]*DashboardLayout, error)
	DeleteLayout(id string) error
}

type SMTPConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	From     string
}

func NewReportService(database *store.DB) *ReportService {
	var db reportStore
	if database != nil {
		db = newDBStore(database)
	}
	cd := time.Duration(alertCooldownMinutes) * time.Minute
	if v := os.Getenv("ALERT_COOLDOWN_MIN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cd = time.Duration(n) * time.Minute
		}
	}
	rs := &ReportService{
		reports:      make(map[string]*ScheduledReport),
		alerts:       make(map[string]*AlertRule),
		alertFiredAt: make(map[string]time.Time),
		stopCh:       make(chan struct{}),
		db:           db,
		cooldownDur:  cd,
	}
	if h := os.Getenv("SMTP_HOST"); h != "" {
		rs.smtpConfig = &SMTPConfig{
			Host:     h,
			Port:     getEnvDefault("SMTP_PORT", "587"),
			User:     os.Getenv("SMTP_USER"),
			Password: os.Getenv("SMTP_PASSWORD"),
			From:     getEnvDefault("SMTP_FROM", os.Getenv("SMTP_USER")),
		}
	}

	// Load persisted reports/alerts
	if db != nil {
		if reports, err := db.LoadReports(); err == nil {
			for _, r := range reports {
				rs.reports[r.ID] = r
			}
			log.Printf("[reports] Loaded %d reports from DB", len(reports))
		}
		if alerts, err := db.LoadAlerts(); err == nil {
			for _, a := range alerts {
				rs.alerts[a.ID] = a
			}
			log.Printf("[reports] Loaded %d alerts from DB", len(alerts))
		}
	}

	if rs.smtpConfig == nil {
		log.Println("[reports] SMTP not configured — email sending disabled")
	}
	return rs
}

func getEnvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func (rs *ReportService) Start(h *Handler) {
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for {
			select {
			case <-ticker.C:
				rs.checkSchedules(h)
				rs.checkAlerts(h)
			case <-rs.stopCh:
				ticker.Stop()
				return
			}
		}
	}()
	log.Println("[reports] Scheduler started (checking every 1m)")
}

func (rs *ReportService) Stop() {
	close(rs.stopCh)
}

func (rs *ReportService) CreateReport(r *ScheduledReport) *ScheduledReport {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	r.ID = newID()
	r.CreatedAt = time.Now().Format(time.RFC3339)
	r.NextRun = rs.computeNextRun(r)
	r.Enabled = true
	rs.reports[r.ID] = r
	if rs.db != nil {
		if err := rs.db.SaveReport(r); err != nil {
			log.Printf("[reports] Failed to persist report %s: %v", r.ID, err)
		}
	}
	return r
}

func (rs *ReportService) UpdateReport(r *ScheduledReport) bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	existing, ok := rs.reports[r.ID]
	if !ok {
		return false
	}
	r.CreatedAt = existing.CreatedAt
	r.NextRun = rs.computeNextRun(r)
	rs.reports[r.ID] = r
	if rs.db != nil {
		if err := rs.db.SaveReport(r); err != nil {
			log.Printf("[reports] Failed to persist report update %s: %v", r.ID, err)
		}
	}
	return true
}

func (rs *ReportService) DeleteReport(id string) bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	_, ok := rs.reports[id]
	if ok {
		delete(rs.reports, id)
		if rs.db != nil {
			if err := rs.db.DeleteReport(id); err != nil {
				log.Printf("[reports] Failed to delete report %s: %v", id, err)
			}
		}
	}
	return ok
}

func (rs *ReportService) GetReport(id string) *ScheduledReport {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.reports[id]
}

func (rs *ReportService) ListReports() []*ScheduledReport {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	list := make([]*ScheduledReport, 0, len(rs.reports))
	for _, r := range rs.reports {
		list = append(list, r)
	}
	return list
}

func (rs *ReportService) CreateAlert(a *AlertRule) *AlertRule {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	a.ID = newID()
	a.CreatedAt = time.Now().Format(time.RFC3339)
	a.Enabled = true
	rs.alerts[a.ID] = a
	if rs.db != nil {
		if err := rs.db.SaveAlert(a); err != nil {
			log.Printf("[reports] Failed to persist alert %s: %v", a.ID, err)
		}
	}
	return a
}

func (rs *ReportService) UpdateAlert(a *AlertRule) bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	_, ok := rs.alerts[a.ID]
	if !ok {
		return false
	}
	rs.alerts[a.ID] = a
	if rs.db != nil {
		if err := rs.db.SaveAlert(a); err != nil {
			log.Printf("[reports] Failed to persist alert update %s: %v", a.ID, err)
		}
	}
	return true
}

func (rs *ReportService) DeleteAlert(id string) bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	_, ok := rs.alerts[id]
	if ok {
		delete(rs.alerts, id)
		if rs.db != nil {
			if err := rs.db.DeleteAlert(id); err != nil {
				log.Printf("[reports] Failed to delete alert %s: %v", id, err)
			}
		}
	}
	return ok
}

func (rs *ReportService) GetAlert(id string) *AlertRule {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.alerts[id]
}

func (rs *ReportService) ListAlerts() []*AlertRule {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	list := make([]*AlertRule, 0, len(rs.alerts))
	for _, a := range rs.alerts {
		list = append(list, a)
	}
	return list
}

func (rs *ReportService) computeNextRun(r *ScheduledReport) string {
	now := time.Now()
	next := now.Add(1 * time.Minute)
	switch r.Frequency {
	case FreqDaily:
		next = time.Date(now.Year(), now.Month(), now.Day(), r.Hour, 0, 0, 0, now.Location())
		if !next.After(now) {
			next = next.Add(24 * time.Hour)
		}
	case FreqWeekly:
		daysUntil := (r.DayOfWeek - int(now.Weekday()) + 7) % 7
		if daysUntil == 0 {
			daysUntil = 7
		}
		next = time.Date(now.Year(), now.Month(), now.Day()+daysUntil, r.Hour, 0, 0, 0, now.Location())
	case FreqMonthly:
		day := r.DayOfMonth
		if day < 1 {
			day = 1
		}
		if day > 28 {
			day = 28
		}
		next = time.Date(now.Year(), now.Month(), day, r.Hour, 0, 0, 0, now.Location())
		if !next.After(now) {
			next = next.AddDate(0, 1, 0)
		}
	}
	return next.Format(time.RFC3339)
}

func (rs *ReportService) checkSchedules(h *Handler) {
	now := time.Now()
	rs.mu.RLock()
	var due []*ScheduledReport
	for _, r := range rs.reports {
		if !r.Enabled {
			continue
		}
		nextRun, err := time.Parse(time.RFC3339, r.NextRun)
		if err != nil || now.Before(nextRun) {
			continue
		}
		due = append(due, r)
	}
	rs.mu.RUnlock()

	for _, r := range due {
		rs.sendReport(h, r)
		rs.mu.Lock()
		r.LastSent = now.Format(time.RFC3339)
		r.NextRun = rs.computeNextRun(r)
		if rs.db != nil {
			rs.db.SaveReport(r)
		}
		rs.mu.Unlock()
	}
}

func (rs *ReportService) sendReport(h *Handler, r *ScheduledReport) {
	log.Printf("[reports] Sending scheduled report %q to %v", r.Name, r.Emails)

	h.mu.RLock()
	summary := fmt.Sprintf("Scheduled Report: %s\nGenerated: %s\nDatasets: %d\n\n",
		r.Name, time.Now().Format(time.RFC3339), len(r.DatasetIDs))
	h.mu.RUnlock()

	emailBody := truncateBody(summary, maxEmailBodyBytes)
	webhookBody := truncateBody(summary, maxWebhookBodyBytes)

	if rs.smtpConfig != nil && len(r.Emails) > 0 {
		rs.sendEmail(r.Emails, fmt.Sprintf("Report: %s", r.Name), emailBody)
	}
	if r.SlackWebhook != "" {
		rs.sendWebhook(r.SlackWebhook, map[string]string{"text": webhookBody})
	}
	if r.TeamsWebhook != "" {
		rs.sendWebhook(r.TeamsWebhook, map[string]interface{}{
			"@type":   "MessageCard",
			"summary": r.Name,
			"title":   fmt.Sprintf("Report: %s", r.Name),
			"text":    webhookBody,
		})
	}
}

func truncateBody(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	truncated := s[:maxBytes]
	// Cut at last newline to avoid mid-line truncation
	if idx := strings.LastIndex(truncated, "\n"); idx > 0 {
		truncated = truncated[:idx]
	}
	return truncated + "\n\n[Message truncated at " + strconv.Itoa(maxBytes) + " bytes]"
}

func (rs *ReportService) checkAlerts(h *Handler) {
	rs.mu.RLock()
	alerts := make([]*AlertRule, 0, len(rs.alerts))
	for _, a := range rs.alerts {
		alerts = append(alerts, a)
	}
	rs.mu.RUnlock()

	for _, a := range alerts {
		if !a.Enabled {
			continue
		}
		rs.evaluateAlert(h, a)
	}
}

func (rs *ReportService) evaluateAlert(h *Handler, a *AlertRule) {
	h.mu.RLock()
	ds, ok := h.datasets[a.DatasetID]
	h.mu.RUnlock()
	if !ok || len(ds.Rows) == 0 {
		return
	}

	metricCol := a.MetricCol
	rows := ds.Rows

	// Compute current and previous aggregate values
	values := make([]float64, 0, len(rows))
	for _, row := range rows {
		if v, err := strconv.ParseFloat(row[metricCol], 64); err == nil {
			values = append(values, v)
		}
	}
	if len(values) == 0 {
		return
	}

	var currentValue, prevValue float64
	// Sum-based: compare last period (latter half) vs previous period (first half)
	mid := len(values) / 2
	if mid == 0 {
		mid = 1
	}
	var sumCurrent, sumPrev float64
	for i, v := range values {
		if i >= mid {
			sumCurrent += v
		} else {
			sumPrev += v
		}
	}
	currentValue = sumCurrent
	prevValue = sumPrev

	var triggered bool
	changePct := 0.0
	if prevValue != 0 {
		changePct = ((currentValue - prevValue) / prevValue) * 100
	}
	switch a.Condition {
	case AlertDrop:
		triggered = changePct < -a.Threshold
	case AlertRise:
		triggered = changePct > a.Threshold
	}

	if triggered {
		rs.mu.RLock()
		lastFired, alreadyFired := rs.alertFiredAt[a.ID]
		rs.mu.RUnlock()
		if alreadyFired && time.Since(lastFired) < rs.cooldownDur {
			log.Printf("[alerts] Alert %q already fired %v ago, skipping (cooldown=%v)", a.Name, time.Since(lastFired).Round(time.Second), rs.cooldownDur)
		} else {
			msg := fmt.Sprintf("Alert: %s\nMetric: %s\nChange: %.2f%%\nThreshold: %.2f%%\nCurrent Period Sum: %.2f\nPrevious Period Sum: %.2f",
				a.Name, metricCol, changePct, a.Threshold, currentValue, prevValue)
			if len(a.Emails) > 0 && rs.smtpConfig != nil {
				rs.sendEmail(a.Emails, fmt.Sprintf("Alert: %s", a.Name), truncateBody(msg, maxEmailBodyBytes))
			}
			if a.SlackHook != "" {
				rs.sendWebhook(a.SlackHook, map[string]string{"text": truncateBody(msg, maxWebhookBodyBytes)})
			}
			log.Printf("[alerts] Triggered alert %q: change=%.2f%% threshold=%.2f%%", a.Name, changePct, a.Threshold)
			rs.mu.Lock()
			rs.alertFiredAt[a.ID] = time.Now()
			rs.mu.Unlock()
		}
	} else {
		rs.mu.Lock()
		delete(rs.alertFiredAt, a.ID)
		rs.mu.Unlock()
	}

	rs.mu.Lock()
	a.LastChecked = time.Now().Format(time.RFC3339)
	if rs.db != nil {
		rs.db.SaveAlert(a)
	}
	rs.mu.Unlock()
}

func (rs *ReportService) sendEmail(to []string, subject, body string) {
	if rs.smtpConfig == nil || rs.smtpConfig.Host == "" {
		log.Println("[email] SMTP not configured, cannot send email")
		return
	}

	go func() {
		body = truncateBody(body, maxEmailBodyBytes)
		msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
			rs.smtpConfig.From, strings.Join(to, ","), subject, body)

		addr := fmt.Sprintf("%s:%s", rs.smtpConfig.Host, rs.smtpConfig.Port)
		auth := smtp.PlainAuth("", rs.smtpConfig.User, rs.smtpConfig.Password, rs.smtpConfig.Host)

		// Try STARTTLS first, then fall back to plain
		if err := sendMailTLS(addr, auth, rs.smtpConfig.From, to, []byte(msg)); err != nil {
			log.Printf("[email] TLS send failed: %v, trying plain", err)
			if err2 := smtp.SendMail(addr, auth, rs.smtpConfig.From, to, []byte(msg)); err2 != nil {
				log.Printf("[email] Plain send also failed: %v", err2)
				return
			}
		}
		log.Printf("[email] Sent to %v subject=%q via %s", to, subject, addr)
	}()
}

func sendMailTLS(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	host := addr
	if idx := strings.IndexByte(addr, ':'); idx > 0 {
		host = addr[:idx]
	}

	tlsConfig := &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("tls dial: %w", err)
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	if err = client.Mail(from); err != nil {
		return fmt.Errorf("smtp mail: %w", err)
	}
	for _, addr := range to {
		if err = client.Rcpt(addr); err != nil {
			return fmt.Errorf("smtp rcpt %s: %w", addr, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err = w.Write(msg); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err = w.Close(); err != nil {
		return fmt.Errorf("smtp close: %w", err)
	}

	return client.Quit()
}

func (rs *ReportService) sendWebhook(url string, payload interface{}) {
	go func() {
		// Truncate any string fields in the payload
		if m, ok := payload.(map[string]string); ok {
			for k, v := range m {
				m[k] = truncateBody(v, maxWebhookBodyBytes)
			}
		} else if m, ok := payload.(map[string]interface{}); ok {
			for k, v := range m {
				if s, ok := v.(string); ok {
					m[k] = truncateBody(s, maxWebhookBodyBytes)
				}
			}
		}
		body, err := json.Marshal(payload)
		if err != nil {
			log.Printf("[webhook] Failed to marshal payload: %v", err)
			return
		}
		// Retry up to 3 times with exponential backoff
		for attempt := 0; attempt < 3; attempt++ {
			if attempt > 0 {
				time.Sleep(time.Duration(1<<uint(attempt)) * time.Second)
			}
			resp, err := http.Post(url, "application/json", bytes.NewReader(body))
			if err != nil {
				log.Printf("[webhook] Attempt %d failed: %v", attempt+1, err)
				continue
			}
			statusOK := resp.StatusCode < 400
			resp.Body.Close()
			if statusOK {
				log.Printf("[webhook] Sent to %s (status %d)", url, resp.StatusCode)
				return
			}
			log.Printf("[webhook] Attempt %d got status %d, retrying...", attempt+1, resp.StatusCode)
		}
		log.Printf("[webhook] All 3 attempts failed for %s", url)
	}()
}

// --- HTTP Handlers ---

func (h *Handler) handleCreateReport(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name         string            `json:"name"`
		DatasetIDs   []string          `json:"datasetIds"`
		Frequency    ScheduleFrequency `json:"frequency"`
		DayOfWeek    int               `json:"dayOfWeek"`
		DayOfMonth   int               `json:"dayOfMonth"`
		Hour         int               `json:"hour"`
		Emails       []string          `json:"emails"`
		SlackWebhook string            `json:"slackWebhook"`
		TeamsWebhook string            `json:"teamsWebhook"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	if body.Name == "" || len(body.DatasetIDs) == 0 {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "name and datasetIds are required"})
		return
	}
	rpt := &ScheduledReport{
		Name:         body.Name,
		DatasetIDs:   body.DatasetIDs,
		Frequency:    body.Frequency,
		DayOfWeek:    body.DayOfWeek,
		DayOfMonth:   body.DayOfMonth,
		Hour:         body.Hour,
		Emails:       body.Emails,
		SlackWebhook: body.SlackWebhook,
		TeamsWebhook: body.TeamsWebhook,
	}
	if rpt.Frequency == "" {
		rpt.Frequency = FreqDaily
	}
	if rpt.Hour < 0 || rpt.Hour > 23 {
		rpt.Hour = 9
	}
	saved := h.reportSvc.CreateReport(rpt)
	SendJSON(w, http.StatusCreated, saved)
}

func (h *Handler) handleListReports(w http.ResponseWriter, r *http.Request) {
	reports := h.reportSvc.ListReports()
	SendJSON(w, http.StatusOK, map[string]interface{}{"reports": reports})
}

func (h *Handler) handleDeleteReport(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "id query parameter required"})
		return
	}
	if !h.reportSvc.DeleteReport(id) {
		SendJSON(w, http.StatusNotFound, map[string]string{"error": "Report not found"})
		return
	}
	SendJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

func (h *Handler) handleCreateAlert(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name      string         `json:"name"`
		DatasetID string         `json:"datasetId"`
		MetricCol string         `json:"metricCol"`
		Condition AlertCondition `json:"condition"`
		Threshold float64        `json:"threshold"`
		Period    string         `json:"period"`
		Emails    []string       `json:"emails"`
		SlackHook string         `json:"slackHook"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	if body.Name == "" || body.DatasetID == "" || body.MetricCol == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "name, datasetId, and metricCol are required"})
		return
	}
	alert := &AlertRule{
		Name:      body.Name,
		DatasetID: body.DatasetID,
		MetricCol: body.MetricCol,
		Condition: body.Condition,
		Threshold: body.Threshold,
		Period:    body.Period,
		Emails:    body.Emails,
		SlackHook: body.SlackHook,
	}
	if alert.Condition == "" {
		alert.Condition = AlertDrop
	}
	if alert.Threshold <= 0 {
		alert.Threshold = 10
	}
	saved := h.reportSvc.CreateAlert(alert)
	SendJSON(w, http.StatusCreated, saved)
}

func (h *Handler) handleListAlerts(w http.ResponseWriter, r *http.Request) {
	alerts := h.reportSvc.ListAlerts()
	SendJSON(w, http.StatusOK, map[string]interface{}{"alerts": alerts})
}

func (h *Handler) handleDeleteAlert(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "id query parameter required"})
		return
	}
	if !h.reportSvc.DeleteAlert(id) {
		SendJSON(w, http.StatusNotFound, map[string]string{"error": "Alert not found"})
		return
	}
	SendJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// --- DB-backed reportStore implementation ---

type dbStore struct {
	db *store.DB
}

func newDBStore(db *store.DB) *dbStore {
	return &dbStore{db: db}
}

func (s *dbStore) SaveReport(r *ScheduledReport) error {
	datasetIDs, _ := json.Marshal(r.DatasetIDs)
	emails, _ := json.Marshal(r.Emails)
	return s.db.SaveReport(r.ID, r.Name, string(datasetIDs), string(r.Frequency), r.DayOfWeek, r.DayOfMonth, r.Hour, string(emails), r.SlackWebhook, r.TeamsWebhook, r.LastSent, r.NextRun, r.Enabled, r.CreatedAt)
}

func (s *dbStore) LoadReports() ([]*ScheduledReport, error) {
	records, err := s.db.LoadReports()
	if err != nil {
		return nil, err
	}
	reports := make([]*ScheduledReport, 0, len(records))
	for _, rec := range records {
		var datasetIDs []string
		json.Unmarshal([]byte(rec.DatasetIDs), &datasetIDs)
		var emails []string
		json.Unmarshal([]byte(rec.Emails), &emails)
		if datasetIDs == nil {
			datasetIDs = []string{}
		}
		if emails == nil {
			emails = []string{}
		}
		reports = append(reports, &ScheduledReport{
			ID:           rec.ID,
			Name:         rec.Name,
			DatasetIDs:   datasetIDs,
			Frequency:    ScheduleFrequency(rec.Frequency),
			DayOfWeek:    rec.DayOfWeek,
			DayOfMonth:   rec.DayOfMonth,
			Hour:         rec.Hour,
			Emails:       emails,
			SlackWebhook: rec.SlackWebhook,
			TeamsWebhook: rec.TeamsWebhook,
			LastSent:     rec.LastSent,
			NextRun:      rec.NextRun,
			Enabled:      rec.Enabled,
			CreatedAt:    rec.CreatedAt,
		})
	}
	return reports, nil
}

func (s *dbStore) DeleteReport(id string) error {
	return s.db.DeleteReport(id)
}

func (s *dbStore) SaveAlert(a *AlertRule) error {
	emails, _ := json.Marshal(a.Emails)
	return s.db.SaveAlert(a.ID, a.Name, a.DatasetID, a.MetricCol, string(a.Condition), a.Threshold, a.Period, string(emails), a.SlackHook, a.Enabled, a.LastChecked, a.CreatedAt)
}

func (s *dbStore) LoadAlerts() ([]*AlertRule, error) {
	records, err := s.db.LoadAlerts()
	if err != nil {
		return nil, err
	}
	alerts := make([]*AlertRule, 0, len(records))
	for _, rec := range records {
		var emails []string
		json.Unmarshal([]byte(rec.Emails), &emails)
		if emails == nil {
			emails = []string{}
		}
		alerts = append(alerts, &AlertRule{
			ID:          rec.ID,
			Name:        rec.Name,
			DatasetID:   rec.DatasetID,
			MetricCol:   rec.MetricCol,
			Condition:   AlertCondition(rec.Condition),
			Threshold:   rec.Threshold,
			Period:      rec.Period,
			Emails:      emails,
			SlackHook:   rec.SlackHook,
			Enabled:     rec.Enabled,
			LastChecked: rec.LastChecked,
			CreatedAt:   rec.CreatedAt,
		})
	}
	return alerts, nil
}

func (s *dbStore) DeleteAlert(id string) error {
	return s.db.DeleteAlert(id)
}

func (s *dbStore) SaveLayout(id string, layout *DashboardLayout) error {
	tilesJSON, _ := json.Marshal(layout.Tiles)
	return s.db.SaveLayout(id, layout.Name, layout.IsDefault, tilesJSON, layout.CreatedAt, layout.UpdatedAt)
}

func (s *dbStore) LoadLayouts() ([]*DashboardLayout, error) {
	records, err := s.db.LoadLayouts()
	if err != nil {
		return nil, err
	}
	layouts := make([]*DashboardLayout, 0, len(records))
	for _, rec := range records {
		var tiles []DashboardTile
		json.Unmarshal([]byte(rec.Tiles), &tiles)
		if tiles == nil {
			tiles = []DashboardTile{}
		}
		layouts = append(layouts, &DashboardLayout{
			ID:        rec.ID,
			Name:      rec.Name,
			IsDefault: rec.IsDefault,
			Tiles:     tiles,
			CreatedAt: rec.CreatedAt,
			UpdatedAt: rec.UpdatedAt,
		})
	}
	return layouts, nil
}

func (s *dbStore) DeleteLayout(id string) error {
	return s.db.DeleteLayout(id)
}
