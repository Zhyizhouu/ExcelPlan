package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"

	"github.com/Zhyizhouu/excelplan/internal/recycle"
	"github.com/Zhyizhouu/excelplan/internal/vault"
	"github.com/Zhyizhouu/excelplan/internal/workbook"
)

type workbookStatus struct {
	Exists bool `json:"exists"`
	// True for a workbook made before the Answer sheet existed.
	Outdated bool   `json:"outdated"`
	Path     string `json:"path"`
}

// handleWorkbookStatus says whether the case's workbook is in the vault and
// whether it predates the Answer sheet, so the page can offer to replace it.
func (s *Server) handleWorkbookStatus(w http.ResponseWriter, r *http.Request) {
	note, err := vault.FindBySlug(s.vaultRoot, r.PathValue("slug"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	out := workbookStatus{Path: workbookPath(note)}
	data, err := os.ReadFile(out.Path)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		s.logger.Warn("reading workbook for status", "path", out.Path, "error", err)
		writeError(w, http.StatusConflict, "Could not read the workbook.")
		return
	default:
		out.Exists = true
		has, err := workbook.HasAnswerSheet(data)
		// An unreadable file is not called outdated: replacing it is only
		// offered for a workbook positively known to be the old template.
		out.Outdated = err == nil && !has
	}
	writeJSON(w, http.StatusOK, out)
}

type replaceRequest struct {
	Confirm bool `json:"confirm"`
}

// handleReplaceWorkbook swaps an old-template workbook for a fresh one.
//
// It is the one route that removes a workbook, so it is fenced three ways: the
// request must say the user confirmed, the file must have no Answer sheet, and
// the old file goes to the Recycle Bin rather than being deleted.
func (s *Server) handleReplaceWorkbook(w http.ResponseWriter, r *http.Request) {
	var body replaceRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !body.Confirm {
		writeError(w, http.StatusBadRequest, "Replacing a workbook needs confirmation.")
		return
	}

	note, example, table, err := s.caseWorkbook(r.PathValue("slug"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	dest := workbookPath(note)

	data, err := os.ReadFile(dest)
	switch {
	case errors.Is(err, os.ErrNotExist):
		writeError(w, http.StatusNotFound, "There is no workbook for this case to replace.")
		return
	case err != nil:
		s.logger.Warn("reading workbook to replace", "path", dest, "error", err)
		writeError(w, http.StatusConflict,
			"Could not read the workbook. Close it in Excel, then try again.")
		return
	}

	has, err := workbook.HasAnswerSheet(data)
	switch {
	case err != nil:
		writeError(w, http.StatusUnprocessableEntity,
			"The workbook in the vault is not a readable .xlsx file, so it was left alone.")
		return
	case has:
		writeError(w, http.StatusConflict,
			"This workbook already has an Answer sheet, so it was left alone.")
		return
	}

	if err := recycle.Move(dest); err != nil {
		s.logger.Warn("recycling workbook", "path", dest, "error", err)
		writeError(w, http.StatusConflict,
			"Could not move the old workbook to the Recycle Bin. Close it in Excel, then try again.")
		return
	}
	s.logger.Info("workbook recycled", "slug", note.Slug, "path", dest)

	n, ok := s.createWorkbook(w, dest, note, example, table)
	if !ok {
		return // createWorkbook has already answered
	}
	writeJSON(w, http.StatusOK, saveResponse{Path: dest, Bytes: n, Created: true})
}
