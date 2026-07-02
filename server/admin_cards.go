package main

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lingqiao/server/internal/audit"
	"github.com/lingqiao/server/internal/features/cards"
	"github.com/lingqiao/server/internal/httpapi"
)

// ── Card Management ──────────────────────────────────────────────────────────
// Note: Core admin card HTTP handlers (List, Generate, Update, Bulk) have been
// migrated to internal/features/cards/handler.go (Phase 3B).
// The following handlers are pending migration or are related to Import/Export.

func (h *AdminHandler) HandleExportCards(w http.ResponseWriter, r *http.Request) {
	cardsList, err := h.cardsSvc.AllCards(r.Context())
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to get cards")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=cards_export.csv")
	w.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(w)
	writer.UseCRLF = true
	_ = writer.Write([]string{"code", "status", "note", "max_sessions", "machine_id", "created_at", "activated_at", "expires_at"})
	for _, c := range cardsList {
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
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		CSV         string `json:"csv"`
		Duration    int    `json:"duration"`
		MaxSessions int    `json:"max_sessions"`
		DryRun      bool   `json:"dry_run"`
	}
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.CSV == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "CSV data is required")
		return
	}
	clampInt(&req.Duration, 1, 17520, 720)
	clampInt(&req.MaxSessions, 1, 100, 1)

	reader := csv.NewReader(strings.NewReader(strings.TrimSpace(req.CSV)))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid CSV: "+err.Error())
		return
	}

	report, err := h.importCardsFromRecords(r.Context(), records, time.Duration(req.Duration)*time.Hour, req.MaxSessions, req.DryRun)
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !req.DryRun && report.Imported > 0 {
		if err := h.auditRecorder.Append(audit.Event{
			Action: "cards_import_completed",
			Detail: fmt.Sprintf("imported=%d duplicates=%d invalid=%d skipped=%d", report.Imported, report.Duplicates, report.Invalid, report.Skipped),
		}); err != nil {
			httpapi.WriteError(w, http.StatusInternalServerError, "Action succeeded but audit log failed to save: "+err.Error())
			return
		}
	}
	if req.DryRun {
		log.Printf("[ADMIN] Previewed card CSV import: valid=%d duplicates=%d invalid=%d", report.ValidRows, report.Duplicates, report.Invalid)
	} else {
		log.Printf("[ADMIN] Imported %d cards from CSV", report.Imported)
	}
	httpapi.WriteOK(w, map[string]interface{}{"imported": report.Imported, "dry_run": req.DryRun, "report": report})
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

func (h *AdminHandler) importCardsFromRecords(ctx context.Context, records [][]string, duration time.Duration, maxSessions int, dryRun bool) (cardImportReport, error) {
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
		if _, exists := seen[normalized]; exists {
			report.Duplicates++
			report.Items = append(report.Items, cardImportItem{Row: rowNumber, Code: formatted, Note: note, Status: "duplicate", Message: "card already exists"})
			continue
		}
		if _, err := h.cardsSvc.GetCard(ctx, formatted); err == nil {
			report.Duplicates++
			report.Items = append(report.Items, cardImportItem{Row: rowNumber, Code: formatted, Note: note, Status: "duplicate", Message: "card already exists"})
			continue
		} else if !errors.Is(err, cards.ErrNotFound) {
			report.Invalid++
			report.Items = append(report.Items, cardImportItem{Row: rowNumber, Code: formatted, Note: note, Status: "error", Message: "database error"})
			continue
		}
		seen[normalized] = struct{}{}
		report.ValidRows++
		item := cardImportItem{Row: rowNumber, Code: formatted, Note: note, Status: "valid"}
		if !dryRun {
			if _, err := h.cardsSvc.CreateWithCode(ctx, formatted, duration, note, maxSessions, ""); err != nil {
				report.Invalid++
				report.ValidRows--
				item.Status = "invalid"
				item.Message = err.Error()
			} else {
				report.Imported++
				item.Status = "imported"
				if err := h.auditRecorder.Append(audit.Event{
					Time:   time.Now(),
					Action: "card_imported",
					Card:   formatted,
					Detail: fmt.Sprintf("duration=%s max_sessions=%d", duration, maxSessions),
				}); err != nil {
					item.Status = "error"
					item.Message = "Audit log failed: " + err.Error()
					report.Items = append(report.Items, item)
					return report, fmt.Errorf("action succeeded partially but audit log failed to save: %w", err)
				}
			}
		}
		report.Items = append(report.Items, item)
	}
	return report, nil
}
