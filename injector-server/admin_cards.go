package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	cardops "github.com/lingqiao/server/internal/cards"
)

// ── Card Management ──────────────────────────────────────────────────────────

func (h *AdminHandler) HandleListCards(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	cards := h.cm.SearchCards(q.Get("search"), q.Get("status"), q.Get("machine"))
	cards = filterCardsAdvanced(cards, q.Get("agent_id"), q.Get("bound"), q.Get("expires_before"), q.Get("expires_after"), q.Get("max_sessions"))
	sort.Slice(cards, func(i, j int) bool {
		return cards[i].CreatedAt.After(cards[j].CreatedAt)
	})
	writeOK(w, map[string]interface{}{"cards": cards})
}

func (h *AdminHandler) HandleGenerateCard(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Duration    int    `json:"duration_hours"`
		MaxSessions int    `json:"max_sessions"`
		Note        string `json:"note"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	clampInt(&req.Duration, 1, 17520, 720)
	clampInt(&req.MaxSessions, 1, 100, 1)
	if len(req.Note) > 200 {
		req.Note = req.Note[:200]
	}

	card, err := h.cm.GenerateCard(time.Duration(req.Duration)*time.Hour, req.Note, req.MaxSessions, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	log.Printf("[ADMIN] Generated card: %s", card.Code)
	writeOK(w, map[string]interface{}{"card": card})
}

func (h *AdminHandler) HandleBatchGenerateCards(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Count       int    `json:"count"`
		Duration    int    `json:"duration_hours"`
		MaxSessions int    `json:"max_sessions"`
		Note        string `json:"note"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	clampInt(&req.Count, 1, 500, 1)
	clampInt(&req.Duration, 1, 17520, 720)
	clampInt(&req.MaxSessions, 1, 100, 1)
	if len(req.Note) > 200 {
		req.Note = req.Note[:200]
	}

	cards, err := h.cm.BatchGenerateCards(req.Count, time.Duration(req.Duration)*time.Hour, req.Note, req.MaxSessions, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	log.Printf("[ADMIN] Batch generated %d cards", len(cards))
	writeOK(w, map[string]interface{}{"cards": cards, "count": len(cards)})
}

func (h *AdminHandler) HandleUpdateCard(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Code        string `json:"code"`
		Action      string `json:"action"`
		ExtendHours int    `json:"extend_hours"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Code == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return
	}

	card := h.cm.GetCard(req.Code)
	if card == nil {
		writeError(w, http.StatusNotFound, "card not found")
		return
	}

	switch req.Action {
	case "disable", "enable", "expire":
		status := CardStatus(req.Action)
		if req.Action == "expire" {
			status = CardExpired
		}
		if err := h.cm.UpdateCardStatus(req.Code, status); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
	case "extend":
		if req.ExtendHours <= 0 {
			req.ExtendHours = 720
		}
		if err := h.cm.ExtendCard(req.Code, time.Duration(req.ExtendHours)*time.Hour); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
	case "unbind":
		if err := h.cm.UnbindCard(req.Code); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "unknown action")
		return
	}

	log.Printf("[ADMIN] Card %s: %s", req.Action, req.Code)
	writeOK(w, nil)
}

func (h *AdminHandler) HandleUpdateCardDetails(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Code        string `json:"code"`
		Note        string `json:"note"`
		MaxSessions int    `json:"max_sessions"`
	}
	body, err := readBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := jsonUnmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Code == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return
	}

	// Detect which fields were explicitly set
	var notePtr *string
	var maxSessPtr *int
	bodyStr := string(body)
	if strings.Contains(bodyStr, "\"note\"") {
		notePtr = &req.Note
	}
	if strings.Contains(bodyStr, "\"max_sessions\"") {
		maxSessPtr = &req.MaxSessions
	}

	if err := h.cm.UpdateCardDetails(req.Code, notePtr, maxSessPtr); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	log.Printf("[ADMIN] Updated card details: %s", req.Code)
	writeOK(w, nil)
}

func (h *AdminHandler) HandleBulkCards(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Codes       []string `json:"codes"`
		Action      string   `json:"action"`
		ExtendHours int      `json:"extend_hours"`
		DryRun      bool     `json:"dry_run"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.Codes) == 0 {
		writeError(w, http.StatusBadRequest, "codes is required")
		return
	}
	if req.Action == "" {
		writeError(w, http.StatusBadRequest, "action is required")
		return
	}

	var (
		result cardops.BulkResult
		err    error
	)
	if req.DryRun {
		result, err = h.cm.PreviewBulkUpdateCardsDetailed(req.Codes, req.Action, req.ExtendHours)
	} else {
		result, err = h.cm.BulkUpdateCardsDetailed(req.Codes, req.Action, req.ExtendHours)
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.DryRun {
		log.Printf("[ADMIN] Bulk %s dry-run: %d cards", req.Action, result.Updated)
	} else {
		log.Printf("[ADMIN] Bulk %s: %d cards", req.Action, result.Updated)
	}
	writeOK(w, map[string]interface{}{"affected": result.Updated, "dry_run": req.DryRun, "result": result})
}

func (h *AdminHandler) HandleExportCards(w http.ResponseWriter, r *http.Request) {
	cards := h.cm.AllCards()
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=cards_export.csv")
	w.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(w)
	writer.UseCRLF = true
	_ = writer.Write([]string{"code", "status", "note", "max_sessions", "machine_id", "created_at", "activated_at", "expires_at"})
	for _, c := range cards {
		activatedAt := ""
		if c.ActivatedAt != nil {
			activatedAt = c.ActivatedAt.Format("2006-01-02 15:04:05")
		}
		_ = writer.Write([]string{
			spreadsheetSafeCSVCell(c.Code),
			spreadsheetSafeCSVCell(string(c.Status)),
			spreadsheetSafeCSVCell(c.Note),
			strconv.Itoa(c.MaxSessions),
			spreadsheetSafeCSVCell(c.MachineID),
			c.CreatedAt.Format("2006-01-02 15:04:05"),
			activatedAt,
			c.ExpiresAt.Format("2006-01-02 15:04:05"),
		})
	}
	writer.Flush()
}

func (h *AdminHandler) HandleImportCards(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		CSV         string `json:"csv"`
		Duration    int    `json:"duration"`
		MaxSessions int    `json:"max_sessions"`
		DryRun      bool   `json:"dry_run"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.CSV == "" {
		writeError(w, http.StatusBadRequest, "CSV data is required")
		return
	}
	clampInt(&req.Duration, 1, 17520, 720)
	clampInt(&req.MaxSessions, 1, 100, 1)

	reader := csv.NewReader(strings.NewReader(strings.TrimSpace(req.CSV)))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid CSV: "+err.Error())
		return
	}

	report := h.importCardsFromRecords(records, time.Duration(req.Duration)*time.Hour, req.MaxSessions, req.DryRun)
	if !req.DryRun && report.Imported > 0 {
		h.cm.RecordAudit(AuditEntry{
			Action: "cards_import_completed",
			Detail: fmt.Sprintf("imported=%d duplicates=%d invalid=%d skipped=%d", report.Imported, report.Duplicates, report.Invalid, report.Skipped),
		})
	}
	if req.DryRun {
		log.Printf("[ADMIN] Previewed card CSV import: valid=%d duplicates=%d invalid=%d", report.ValidRows, report.Duplicates, report.Invalid)
	} else {
		log.Printf("[ADMIN] Imported %d cards from CSV", report.Imported)
	}
	writeOK(w, map[string]interface{}{"imported": report.Imported, "dry_run": req.DryRun, "report": report})
}

type cardImportItem struct {
	Row     int    `json:"row"`
	Code    string `json:"code,omitempty"`
	Note    string `json:"note,omitempty"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type cardImportReport struct {
	TotalRows  int              `json:"total_rows"`
	ValidRows  int              `json:"valid_rows"`
	Imported   int              `json:"imported"`
	Duplicates int              `json:"duplicates"`
	Invalid    int              `json:"invalid"`
	Skipped    int              `json:"skipped"`
	Items      []cardImportItem `json:"items"`
}

func (h *AdminHandler) importCardsFromRecords(records [][]string, duration time.Duration, maxSessions int, dryRun bool) cardImportReport {
	report := cardImportReport{Items: make([]cardImportItem, 0, len(records))}
	seen := make(map[string]struct{}, len(records))
	for idx, record := range records {
		rowNumber := idx + 1
		if len(record) == 0 {
			continue
		}
		code := strings.TrimSpace(record[0])
		lower := strings.ToLower(code)
		if lower == "code" || strings.Contains(lower, "卡密") {
			continue
		}

		report.TotalRows++
		if code == "" {
			report.Skipped++
			report.Items = append(report.Items, cardImportItem{Row: rowNumber, Status: "skipped", Message: "empty code"})
			continue
		}

		note := "imported"
		if len(record) > 1 && strings.TrimSpace(record[1]) != "" {
			note = strings.TrimSpace(record[1])
		}
		normalized, err := validateImportCardCode(code)
		if err != nil {
			report.Invalid++
			report.Items = append(report.Items, cardImportItem{Row: rowNumber, Code: code, Note: note, Status: "invalid", Message: err.Error()})
			continue
		}
		formatted := FormatCardCode(normalized)
		if _, exists := seen[normalized]; exists || h.cm.GetCard(formatted) != nil {
			report.Duplicates++
			report.Items = append(report.Items, cardImportItem{Row: rowNumber, Code: formatted, Note: note, Status: "duplicate", Message: "card already exists"})
			continue
		}
		seen[normalized] = struct{}{}
		report.ValidRows++
		item := cardImportItem{Row: rowNumber, Code: formatted, Note: note, Status: "valid"}
		if !dryRun {
			if _, err := h.cm.GenerateCardWithCode(formatted, duration, note, maxSessions, ""); err != nil {
				report.Invalid++
				report.ValidRows--
				item.Status = "invalid"
				item.Message = err.Error()
			} else {
				report.Imported++
				item.Status = "imported"
			}
		}
		report.Items = append(report.Items, item)
	}
	return report
}

