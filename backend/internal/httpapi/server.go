// Package httpapi serves the study plan to the browser, and ticks cases off
// in it.
//
// Reading is the bulk of it, but one route writes: ticking a case sets the
// checkbox in the note itself, so the vault stays the single source of truth
// rather than this app keeping a competing record of what is done. Obsidian
// and ExcelPlan therefore edit the same files, and whichever was touched last
// is simply what is true.
//
// The write is a single-character substitution guarded by vault.SetComplete —
// see the comment there for why nothing else in the file is ever touched.
package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/Zhyizhouu/excelplan/internal/datasets"
	"github.com/Zhyizhouu/excelplan/internal/progresslog"
	"github.com/Zhyizhouu/excelplan/internal/vault"
)

type Server struct {
	vaultRoot   string
	hubNotePath string
	log         *progresslog.Log
	logger      *slog.Logger
}

func New(vaultRoot, hubNotePath string, log *progresslog.Log, logger *slog.Logger) *Server {
	return &Server{vaultRoot: vaultRoot, hubNotePath: hubNotePath, log: log, logger: logger}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /v1/progress", s.handleProgress)
	mux.HandleFunc("GET /v1/notes/{slug}", s.handleNote)
	mux.HandleFunc("POST /v1/notes/{slug}/complete", s.handleSetComplete)
	mux.HandleFunc("GET /v1/notes/{slug}/workbook.xlsx", s.handleDownloadWorkbook)
	mux.HandleFunc("GET /v1/notes/{slug}/data.csv", s.handleCSV)
	mux.HandleFunc("POST /v1/notes/{slug}/workbook", s.handleSaveWorkbook)
	return withCORS(withLogging(mux, s.logger))
}

// noteResponse is everything the detail page draws: the case itself, the data
// it works against, and the answer it should produce.
//
// Example and Dataset are omitted when a case has no worked example yet, which
// is most of them — the page then says so rather than drawing an empty table
// that reads like a failure.
type noteResponse struct {
	Note    vault.Note        `json:"note"`
	Example *datasets.Example `json:"example,omitempty"`
	Dataset *datasets.Table   `json:"dataset,omitempty"`
}

func (s *Server) handleNote(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	note, err := vault.FindBySlug(s.vaultRoot, slug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	out := noteResponse{Note: note}
	if example, ok := datasets.ExampleFor(note.Slug); ok {
		out.Example = &example
		if table, ok := datasets.Get(example.DatasetID); ok {
			// Only the columns this case actually uses. A 41-row sheet with
			// eleven columns is the thing the case is teaching you to narrow
			// down, so handing it over whole would be doing the reading for
			// them — and would not fit on screen anyway.
			out.Dataset = pick(table, example.InputColumns)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// pick narrows a table to the named columns, preserving row order.
func pick(t datasets.Table, columns []string) *datasets.Table {
	if len(columns) == 0 {
		return &t
	}
	rows := make([]map[string]any, 0, len(t.Rows))
	for _, row := range t.Rows {
		trimmed := make(map[string]any, len(columns))
		for _, c := range columns {
			if v, ok := row[c]; ok {
				trimmed[c] = v
			}
		}
		rows = append(rows, trimmed)
	}
	out := t
	out.Columns = columns
	out.Rows = rows
	return &out
}

// progressResponse is the whole dashboard in one payload — every screen this
// app has renders from one call, because the plan is small enough (110 notes)
// that splitting it into paginated or per-phase requests would only add
// round trips for no real saving.
type progressResponse struct {
	Completed int                   `json:"completed"`
	Total     int                   `json:"total"`
	NextUp    *vault.Note           `json:"nextUp,omitempty"`
	Phases    []vault.PhaseProgress `json:"phases"`
	Streak    progresslog.Streak    `json:"streak"`
}

func (s *Server) handleProgress(w http.ResponseWriter, r *http.Request) {
	s.respondProgress(w)
}

type setCompleteRequest struct {
	Complete bool `json:"complete"`
}

// handleSetComplete ticks or unticks one case, then answers with the whole
// refreshed plan.
//
// Returning full progress rather than an acknowledgement is deliberate: one
// tick moves the week total, the phase total, the overall percentage, the
// streak, and which case comes next. Making the client re-derive all of that,
// or fetch it separately, is how the number on screen and the number in the
// file drift apart.
func (s *Server) handleSetComplete(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	var body setCompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest,
			"Expected a JSON body with a `complete` field.")
		return
	}

	// Resolved by slug, never by a path from the client. The note's location
	// is the server's business, and accepting a path would be a traversal hole.
	note, err := vault.FindBySlug(s.vaultRoot, slug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	if _, err := vault.SetComplete(note.Path, body.Complete); err != nil {
		s.logger.Error("setting completion", "slug", slug, "error", err)
		writeError(w, http.StatusInternalServerError,
			"Could not update the note. It may be locked by another program.")
		return
	}

	s.logger.Info("completion set", "slug", slug, "complete", body.Complete)
	s.respondProgress(w)
}

// respondProgress reads the vault and writes the whole plan.
//
// Shared by the GET and by the tick, so both answer from a fresh read of the
// files rather than one of them trusting an in-memory copy.
func (s *Server) respondProgress(w http.ResponseWriter) {
	notes, err := vault.ReadAll(s.vaultRoot)
	if err != nil {
		s.logger.Error("reading vault", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not read the study plan.")
		return
	}
	phases, err := vault.ReadPhases(s.hubNotePath)
	if err != nil {
		s.logger.Warn("reading phase names, continuing with numbers only", "error", err)
	}

	now := time.Now()
	if s.log.Observe(notes, now) {
		if err := s.log.Save(); err != nil {
			// The response is still correct — Observe already updated the
			// in-memory log — only tomorrow's streak read is at risk if the
			// process dies before the next successful save.
			s.logger.Error("saving progress log", "error", err)
		}
	}

	progress := vault.Roll(notes, phases)
	writeJSON(w, http.StatusOK, progressResponse{
		Completed: progress.Completed,
		Total:     progress.Total,
		NextUp:    progress.NextUp,
		Phases:    progress.Phases,
		Streak:    progresslog.Compute(s.log, now),
	})
}

func withLogging(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		logger.Info("request", "method", r.Method, "path", r.URL.Path)
	})
}

// withCORS allows the Vite dev server's origin. This app is one person's
// local tool, never deployed, so the allowlist is deliberately small rather
// than configurable — there is no second origin it ever needs to trust.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
