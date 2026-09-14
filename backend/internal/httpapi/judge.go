package httpapi

import (
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/Zhyizhouu/excelplan/internal/datasets"
	"github.com/Zhyizhouu/excelplan/internal/judge"
	"github.com/Zhyizhouu/excelplan/internal/vault"
)

type judgeResponse struct {
	judge.Report
	Path string `json:"path"`
	// When the file was last saved, so "you forgot Ctrl+S" is visible on the
	// page instead of reading as a wrong answer.
	SavedAt time.Time `json:"savedAt"`
}

// handleJudge checks the case's workbook in the vault against its worked
// example. It reads the file as last saved; unsaved edits in Excel are not seen.
func (s *Server) handleJudge(w http.ResponseWriter, r *http.Request) {
	note, err := vault.FindBySlug(s.vaultRoot, r.PathValue("slug"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if note.Week == 1 {
		writeError(w, http.StatusUnprocessableEntity, "Week 1 is practice and is not checked.")
		return
	}
	example, ok := datasets.ExampleFor(note.Slug)
	if !ok {
		writeError(w, http.StatusNotFound,
			"This case has no worked example yet, so there is nothing to check against.")
		return
	}
	if example.Reference {
		writeError(w, http.StatusUnprocessableEntity,
			"This case's example is a reference. Its answer is not something the checker can compare.")
		return
	}

	path := workbookPath(note)
	info, err := os.Stat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		writeError(w, http.StatusNotFound,
			"There is no workbook for this case yet. Use Open in Excel, build your answer, save, then check.")
		return
	case err != nil:
		s.logger.Error("checking workbook to judge", "path", path, "error", err)
		writeError(w, http.StatusInternalServerError, "Could not find the workbook.")
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		s.logger.Warn("reading workbook to judge", "path", path, "error", err)
		writeError(w, http.StatusConflict,
			"Could not read the workbook. If Excel is in the middle of saving, try again in a moment.")
		return
	}

	report, err := judge.Judge(data, example)
	if err != nil {
		s.logger.Warn("judging workbook", "path", path, "error", err)
		writeError(w, http.StatusUnprocessableEntity,
			"The workbook in the vault is not a readable .xlsx file.")
		return
	}

	s.logger.Info("judged", "slug", note.Slug, "passed", report.Passed)
	writeJSON(w, http.StatusOK, judgeResponse{Report: report, Path: path, SavedAt: info.ModTime()})
}
