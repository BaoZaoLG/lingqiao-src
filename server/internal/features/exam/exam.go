package exam

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/lingqiao/server/internal/auth"
	"github.com/lingqiao/server/internal/features/sessions"
	"github.com/lingqiao/server/internal/httpapi"
)

// ── Model ────────────────────────────────────────────────────────────────────

// Record represents a single exam question answer event.
type Record struct {
	ID             int64     `json:"id"`
	CardCode       string    `json:"card_code"`
	MachineID      string    `json:"machine_id"`
	ExamID         string    `json:"exam_id"`
	CourseID       string    `json:"course_id"`
	ClassID        string    `json:"class_id"`
	QuestionID     string    `json:"question_id"`
	QuestionText   string    `json:"question_text"`
	QuestionType   string    `json:"question_type"`
	OptionsJSON    string    `json:"options_json,omitempty"`
	ProvidedAnswer string    `json:"provided_answer"`
	CorrectAnswer  string    `json:"correct_answer,omitempty"`
	ModelName      string    `json:"model_name"`
	Confidence     float64   `json:"confidence"`
	AnswerTimeMs   int64     `json:"answer_time_ms"`
	FillOK         bool      `json:"fill_ok"`
	CreatedAt      time.Time `json:"created_at"`
}

// ── Repository ───────────────────────────────────────────────────────────────

// Repository handles persistence for exam records.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new exam record repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

const insertRecordSQL = `INSERT INTO exam_records
	(card_code, machine_id, exam_id, course_id, class_id,
	 question_id, question_text, question_type, options_json,
	 provided_answer, correct_answer, model_name, confidence,
	 answer_time_ms, fill_ok, created_at)
	VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`

func recordArgs(rec *Record) []any {
	return []any{
		rec.CardCode, rec.MachineID, rec.ExamID, rec.CourseID, rec.ClassID,
		rec.QuestionID, rec.QuestionText, rec.QuestionType, rec.OptionsJSON,
		rec.ProvidedAnswer, rec.CorrectAnswer, rec.ModelName, rec.Confidence,
		rec.AnswerTimeMs, boolToInt(rec.FillOK), rec.CreatedAt,
	}
}

// Insert stores a new exam record.
func (r *Repository) Insert(ctx context.Context, rec *Record) error {
	if _, err := r.db.ExecContext(ctx, insertRecordSQL, recordArgs(rec)...); err != nil {
		return fmt.Errorf("insert exam record: %w", err)
	}
	return nil
}

// InsertBatch atomically stores all exam records or none of them.
func (r *Repository) InsertBatch(ctx context.Context, records []Record) error {
	if len(records) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin exam batch: %w", err)
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, insertRecordSQL)
	if err != nil {
		return fmt.Errorf("prepare exam batch: %w", err)
	}
	defer stmt.Close()
	for i := range records {
		if _, err := stmt.ExecContext(ctx, recordArgs(&records[i])...); err != nil {
			return fmt.Errorf("insert exam record %d: %w", i, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit exam batch: %w", err)
	}
	return nil
}

// Filter holds query parameters for listing exam records.
type Filter struct {
	CardCode string
	ExamID   string
	Question string
	Page     int
	PageSize int
}

// List returns paginated exam records matching the filter.
func (r *Repository) List(ctx context.Context, f Filter) ([]Record, int, error) {
	where := ""
	args := []any{}
	if f.CardCode != "" {
		where += " AND card_code = ?"
		args = append(args, f.CardCode)
	}
	if f.ExamID != "" {
		where += " AND exam_id = ?"
		args = append(args, f.ExamID)
	}
	if f.Question != "" {
		where += " AND (question_text LIKE ? OR question_id = ?)"
		args = append(args, "%"+f.Question+"%", f.Question)
	}
	if len(where) > 0 {
		where = "WHERE " + where[5:] // strip leading " AND "
	}

	// Count total
	var total int
	countSQL := "SELECT COUNT(*) FROM exam_records " + where
	if err := r.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count exam records: %w", err)
	}

	// Paginated query
	page := f.Page
	if page < 1 {
		page = 1
	}
	pageSize := f.PageSize
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize

	query := `SELECT id, card_code, machine_id, exam_id, course_id, class_id,
		question_id, question_text, question_type, options_json,
		provided_answer, correct_answer, model_name, confidence,
		answer_time_ms, fill_ok, created_at
		FROM exam_records ` + where + ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	queryArgs := append(args, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query exam records: %w", err)
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var rec Record
		var fillOK int
		if err := rows.Scan(
			&rec.ID, &rec.CardCode, &rec.MachineID, &rec.ExamID, &rec.CourseID, &rec.ClassID,
			&rec.QuestionID, &rec.QuestionText, &rec.QuestionType, &rec.OptionsJSON,
			&rec.ProvidedAnswer, &rec.CorrectAnswer, &rec.ModelName, &rec.Confidence,
			&rec.AnswerTimeMs, &fillOK, &rec.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan exam record: %w", err)
		}
		rec.FillOK = fillOK != 0
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate exam records: %w", err)
	}
	return records, total, nil
}

// Stats returns aggregate statistics for exam records.
func (r *Repository) Stats(ctx context.Context) (map[string]any, error) {
	var total, filled, correct int
	err := r.db.QueryRowContext(ctx, `SELECT
		COUNT(*),
		COALESCE(SUM(fill_ok), 0),
		COALESCE(SUM(CASE WHEN correct_answer != '' AND correct_answer = provided_answer THEN 1 ELSE 0 END), 0)
		FROM exam_records`).Scan(&total, &filled, &correct)
	if err != nil {
		return nil, fmt.Errorf("exam stats: %w", err)
	}
	return map[string]any{
		"total":   total,
		"filled":  filled,
		"correct": correct,
	}, nil
}

// ListAll returns all exam records matching the filter without pagination.
// Used for CSV export.
func (r *Repository) ListAll(ctx context.Context, f Filter) ([]Record, error) {
	where := ""
	args := []any{}
	if f.CardCode != "" {
		where += " AND card_code = ?"
		args = append(args, f.CardCode)
	}
	if f.ExamID != "" {
		where += " AND exam_id = ?"
		args = append(args, f.ExamID)
	}
	if f.Question != "" {
		where += " AND (question_text LIKE ? OR question_id = ?)"
		args = append(args, "%"+f.Question+"%", f.Question)
	}
	if len(where) > 0 {
		where = "WHERE " + where[5:]
	}

	query := `SELECT id, card_code, machine_id, exam_id, course_id, class_id,
		question_id, question_text, question_type, options_json,
		provided_answer, correct_answer, model_name, confidence,
		answer_time_ms, fill_ok, created_at
		FROM exam_records ` + where + ` ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query exam records for export: %w", err)
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var rec Record
		var fillOK int
		if err := rows.Scan(
			&rec.ID, &rec.CardCode, &rec.MachineID, &rec.ExamID, &rec.CourseID, &rec.ClassID,
			&rec.QuestionID, &rec.QuestionText, &rec.QuestionType, &rec.OptionsJSON,
			&rec.ProvidedAnswer, &rec.CorrectAnswer, &rec.ModelName, &rec.Confidence,
			&rec.AnswerTimeMs, &fillOK, &rec.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan exam record: %w", err)
		}
		rec.FillOK = fillOK != 0
		records = append(records, rec)
	}
	return records, rows.Err()
}

// ── Handler ──────────────────────────────────────────────────────────────────

// Handler exposes HTTP endpoints for exam record reporting and querying.
type Handler struct {
	repo        *Repository
	sessionsSvc *sessions.Service
}

// NewHandler creates a new exam handler.
func NewHandler(repo *Repository, sessionsSvc *sessions.Service) *Handler {
	return &Handler{repo: repo, sessionsSvc: sessionsSvc}
}

// reportRequest is the JSON body sent by the AutoExam.js client.
type reportRequest struct {
	SessionToken string `json:"session_token"`
	ExamID       string `json:"exam_id"`
	CourseID     string `json:"course_id"`
	ClassID      string `json:"class_id"`
	Questions    []struct {
		QuestionID     string  `json:"question_id"`
		QuestionText   string  `json:"question_text"`
		QuestionType   string  `json:"question_type"`
		OptionsJSON    string  `json:"options_json,omitempty"`
		ProvidedAnswer string  `json:"provided_answer"`
		CorrectAnswer  string  `json:"correct_answer,omitempty"`
		ModelName      string  `json:"model_name"`
		Confidence     float64 `json:"confidence"`
		AnswerTimeMs   int64   `json:"answer_time_ms"`
		FillOK         bool    `json:"fill_ok"`
	} `json:"questions"`
}

func recordsFromReport(req reportRequest, sess sessions.Session, now time.Time) []Record {
	records := make([]Record, 0, len(req.Questions))
	for _, q := range req.Questions {
		records = append(records, Record{
			CardCode: sess.CardCode, MachineID: sess.MachineID, ExamID: req.ExamID,
			CourseID: req.CourseID, ClassID: req.ClassID, QuestionID: q.QuestionID,
			QuestionText: q.QuestionText, QuestionType: q.QuestionType, OptionsJSON: q.OptionsJSON,
			ProvidedAnswer: q.ProvidedAnswer, CorrectAnswer: q.CorrectAnswer, ModelName: q.ModelName,
			Confidence: q.Confidence, AnswerTimeMs: q.AnswerTimeMs, FillOK: q.FillOK, CreatedAt: now,
		})
	}
	return records
}

// HandleReport accepts batch exam record reports from the client.
// POST /api/v1/exam/report  (HMAC + session authenticated)
func (h *Handler) HandleReport(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}

	body, err := httpapi.ReadBody(r)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := auth.VerifyRequest(r, string(body)); err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}

	var req reportRequest
	if err := json.Unmarshal(body, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid json")
		return
	}

	sess, err := h.sessionsSvc.ValidateActiveToken(r.Context(), req.SessionToken)
	if err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, "invalid session")
		return
	}

	records := recordsFromReport(req, sess, time.Now())
	if err := h.repo.InsertBatch(r.Context(), records); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to save exam records")
		return
	}

	httpapi.WriteOK(w, map[string]any{"inserted": len(records)})
}

// collectRequest is the JSON body sent by AutoExam.js (page context, no HMAC).
type collectRequest struct {
	SessionToken string `json:"session_token"`
	ExamID       string `json:"exam_id"`
	CourseID     string `json:"course_id"`
	ClassID      string `json:"class_id"`
	Questions    []struct {
		QuestionID     string  `json:"question_id"`
		QuestionText   string  `json:"question_text"`
		QuestionType   string  `json:"question_type"`
		OptionsJSON    string  `json:"options_json,omitempty"`
		ProvidedAnswer string  `json:"provided_answer"`
		CorrectAnswer  string  `json:"correct_answer,omitempty"`
		ModelName      string  `json:"model_name"`
		Confidence     float64 `json:"confidence"`
		AnswerTimeMs   int64   `json:"answer_time_ms"`
		FillOK         bool    `json:"fill_ok"`
	} `json:"questions"`
}

func recordsFromCollect(req collectRequest, sess sessions.Session, now time.Time) []Record {
	records := make([]Record, 0, len(req.Questions))
	for _, q := range req.Questions {
		records = append(records, Record{
			CardCode: sess.CardCode, MachineID: sess.MachineID, ExamID: req.ExamID,
			CourseID: req.CourseID, ClassID: req.ClassID, QuestionID: q.QuestionID,
			QuestionText: q.QuestionText, QuestionType: q.QuestionType, OptionsJSON: q.OptionsJSON,
			ProvidedAnswer: q.ProvidedAnswer, CorrectAnswer: q.CorrectAnswer, ModelName: q.ModelName,
			Confidence: q.Confidence, AnswerTimeMs: q.AnswerTimeMs, FillOK: q.FillOK, CreatedAt: now,
		})
	}
	return records
}

// HandleCollect accepts exam records from the page context (no HMAC required).
// The session token is validated against the sessions table to authenticate.
// POST /api/v1/exam/collect
func (h *Handler) HandleCollect(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}

	var req collectRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if req.SessionToken == "" {
		httpapi.WriteError(w, http.StatusUnauthorized, "session_token required")
		return
	}

	// Validate session token (lookup only, no machine/card binding check)
	sess, err := h.sessionsSvc.ValidateActiveToken(r.Context(), req.SessionToken)
	if err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, "invalid session")
		return
	}

	records := recordsFromCollect(req, sess, time.Now())
	if err := h.repo.InsertBatch(r.Context(), records); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to save exam records")
		return
	}

	httpapi.WriteOK(w, map[string]any{"inserted": len(records)})
}

// HandleAdminList returns paginated exam records for the admin panel.
// GET /admin/api/exam/records
func (h *Handler) HandleAdminList(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodGet) {
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}

	records, total, err := h.repo.List(r.Context(), Filter{
		CardCode: r.URL.Query().Get("card_code"),
		ExamID:   r.URL.Query().Get("exam_id"),
		Question: r.URL.Query().Get("q"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to list exam records")
		return
	}

	httpapi.WriteOK(w, map[string]any{
		"records":   records,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// HandleAdminStats returns aggregate exam statistics for the admin panel.
// GET /admin/api/exam/stats
func (h *Handler) HandleAdminStats(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodGet) {
		return
	}
	stats, err := h.repo.Stats(r.Context())
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to load exam stats")
		return
	}
	httpapi.WriteOK(w, stats)
}

// HandleAdminExport streams all matching exam records as a CSV download.
// GET /admin/api/exam/export?card_code=&exam_id=&q=
func (h *Handler) HandleAdminExport(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodGet) {
		return
	}

	records, err := h.repo.ListAll(r.Context(), Filter{
		CardCode: r.URL.Query().Get("card_code"),
		ExamID:   r.URL.Query().Get("exam_id"),
		Question: r.URL.Query().Get("q"),
	})
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to export exam records")
		return
	}

	filename := "exam_records_" + time.Now().Format("20060102_150405") + ".csv"
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")

	cw := csv.NewWriter(w)
	defer cw.Flush()

	// BOM for Excel UTF-8 compatibility
	w.Write([]byte{0xEF, 0xBB, 0xBF})

	// Header row
	if err := cw.Write([]string{
		"ID", "卡号", "机器ID", "考试ID", "课程ID", "班级ID",
		"题目ID", "题目文本", "题型", "选项JSON",
		"提交答案", "正确答案", "模型", "置信度",
		"答题耗时(ms)", "填充成功", "时间",
	}); err != nil {
		return
	}

	for _, rec := range records {
		if err := cw.Write([]string{
			strconv.FormatInt(rec.ID, 10),
			spreadsheetSafe(rec.CardCode),
			spreadsheetSafe(rec.MachineID),
			spreadsheetSafe(rec.ExamID),
			spreadsheetSafe(rec.CourseID),
			spreadsheetSafe(rec.ClassID),
			spreadsheetSafe(rec.QuestionID),
			spreadsheetSafe(rec.QuestionText),
			spreadsheetSafe(rec.QuestionType),
			spreadsheetSafe(rec.OptionsJSON),
			spreadsheetSafe(rec.ProvidedAnswer),
			spreadsheetSafe(rec.CorrectAnswer),
			spreadsheetSafe(rec.ModelName),
			strconv.FormatFloat(rec.Confidence, 'f', 4, 64),
			strconv.FormatInt(rec.AnswerTimeMs, 10),
			boolToStr(rec.FillOK),
			rec.CreatedAt.Format("2006-01-02 15:04:05"),
		}); err != nil {
			return
		}
	}
}

func boolToStr(b bool) string {
	if b {
		return "是"
	}
	return "否"
}

func spreadsheetSafe(value string) string {
	if value == "" {
		return value
	}
	switch value[0] {
	case '=', '+', '-', '@':
		return "'" + value
	default:
		return value
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
