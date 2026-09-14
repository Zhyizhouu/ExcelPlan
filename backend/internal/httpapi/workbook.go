package httpapi

import (
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Zhyizhouu/excelplan/internal/datasets"
	"github.com/Zhyizhouu/excelplan/internal/openfile"
	"github.com/Zhyizhouu/excelplan/internal/vault"
	"github.com/Zhyizhouu/excelplan/internal/workbook"
)

// workbookPath is where a case's workbook lives: beside its note. Derived from
// the note's own path rather than rebuilt from a phase number, which would
// guess wrong on the merged "2-3" folder.
func workbookPath(note vault.Note) string {
	return filepath.Join(filepath.Dir(note.Path), workbook.Filename(note))
}

// caseWorkbook assembles the pieces a case's exports need.
func (s *Server) caseWorkbook(slug string) (vault.Note, datasets.Example, *datasets.Table, error) {
	note, err := vault.FindBySlug(s.vaultRoot, slug)
	if err != nil {
		return vault.Note{}, datasets.Example{}, nil, err
	}
	example, _ := datasets.ExampleFor(note.Slug) // zero value is usable
	var table *datasets.Table
	if t, ok := datasets.Get(example.DatasetID); ok {
		table = pick(t, example.InputColumns)
	}
	return note, example, table, nil
}

// handleDownloadWorkbook streams the .xlsx to the browser.
func (s *Server) handleDownloadWorkbook(w http.ResponseWriter, r *http.Request) {
	note, example, table, err := s.caseWorkbook(r.PathValue("slug"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	data, err := workbook.Build(note, example, table)
	if err != nil {
		s.logger.Error("building workbook", "slug", note.Slug, "error", err)
		writeError(w, http.StatusInternalServerError, "Could not build the workbook.")
		return
	}

	name := workbook.Filename(note)
	w.Header().Set("Content-Type",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	// Quoted because the name contains spaces; the browser uses it verbatim.
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s"`, name))
	_, _ = w.Write(data)
}

// handleCSV returns the input table as CSV text, for pasting into a sheet that
// is already open. Plain text rather than a download, because the point is the
// clipboard.
func (s *Server) handleCSV(w http.ResponseWriter, r *http.Request) {
	_, _, table, err := s.caseWorkbook(r.PathValue("slug"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if table == nil {
		writeError(w, http.StatusNotFound, "This case has no dataset yet.")
		return
	}

	var sb strings.Builder
	cw := csv.NewWriter(&sb)
	_ = cw.Write(table.Columns)
	for _, row := range table.Rows {
		record := make([]string, len(table.Columns))
		for i, col := range table.Columns {
			if v, ok := row[col]; ok && v != nil {
				record[i] = fmt.Sprintf("%v", v)
			}
		}
		_ = cw.Write(record)
	}
	cw.Flush()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(sb.String()))
}

type saveResponse struct {
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
	// False when the workbook was already there and was left exactly as it is.
	// The page needs the difference: "saved" and "already yours, untouched" are
	// not the same sentence, and only one of them should sound like new work.
	Created bool `json:"created"`
	// Whether Excel was launched on it. Reported rather than assumed, so the
	// page can say "open it yourself" on the machine where that fails instead
	// of claiming a window appeared that did not.
	Opened bool `json:"opened"`
}

// handleSaveWorkbook makes sure the case's workbook is in its phase folder,
// and — with ?open=1 — opens it.
//
// The one rule: an existing workbook is never written to. That file is where
// the answers get typed, and replacing it is the single unrecoverable thing
// this app could do. So a second request does not overwrite it, and does not
// refuse either; it opens what is already there. Both requests mean "get me
// into this case", and after the first one that is a file that already exists
// — treating the normal second visit as a conflict was making the safe path
// feel like an error.
func (s *Server) handleSaveWorkbook(w http.ResponseWriter, r *http.Request) {
	note, example, table, err := s.caseWorkbook(r.PathValue("slug"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	dest := workbookPath(note)

	var (
		size    int64
		created bool
	)
	switch info, err := os.Stat(dest); {
	case err == nil:
		if info.IsDir() {
			writeError(w, http.StatusConflict, fmt.Sprintf(
				"%s is a folder, not a workbook. Move it out of the way first.",
				filepath.Base(dest)))
			return
		}
		size = info.Size() // left untouched

	case errors.Is(err, os.ErrNotExist):
		n, ok := s.createWorkbook(w, dest, note, example, table)
		if !ok {
			return // createWorkbook has already answered
		}
		size, created = n, true

	default:
		s.logger.Error("checking destination", "path", dest, "error", err)
		writeError(w, http.StatusInternalServerError, "Could not check the destination.")
		return
	}

	// Straight into Excel, when asked. Saving to the vault is not an archiving
	// step — it is how a case gets started — and going to find the file
	// afterwards is the kind of small friction that ends a daily habit around
	// week three.
	opened := false
	if r.URL.Query().Get("open") == "1" {
		if err := openfile.Open(dest); err != nil {
			// Never fatal. The workbook is on disk and correct; only the
			// convenience failed, and reporting that as a failed save would
			// send someone looking for a file that is already there.
			s.logger.Warn("opening workbook", "path", dest, "error", err)
		} else {
			opened = true
		}
	}

	s.logger.Info("workbook ready", "slug", note.Slug, "path", dest, "bytes", size,
		"created", created, "opened", opened)
	writeJSON(w, http.StatusOK, saveResponse{
		Path: dest, Bytes: size, Created: created, Opened: opened,
	})
}

// createWorkbook builds the workbook and writes it to dest, which must not
// exist yet. Reports its own failures to w, and returns ok=false when it has —
// the three ways this fails need three different things said, but the caller
// does nothing different for any of them.
func (s *Server) createWorkbook(
	w http.ResponseWriter,
	dest string,
	note vault.Note,
	example datasets.Example,
	table *datasets.Table,
) (n int64, ok bool) {
	data, err := workbook.Build(note, example, table)
	if err != nil {
		s.logger.Error("building workbook", "slug", note.Slug, "error", err)
		writeError(w, http.StatusInternalServerError, "Could not build the workbook.")
		return 0, false
	}

	// O_EXCL so the create fails rather than truncates, if something wrote the
	// file in the moment between the caller's Stat and now. This is the line
	// that makes "never overwrite" true rather than merely likely.
	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		s.logger.Error("creating workbook", "path", dest, "error", err)
		writeError(w, http.StatusInternalServerError,
			"Could not write into the vault folder.")
		return 0, false
	}

	written, err := f.Write(data)
	if err != nil {
		_ = f.Close()
		s.logger.Error("writing workbook", "path", dest, "error", err)
		writeError(w, http.StatusInternalServerError, "Could not finish writing the file.")
		return 0, false
	}

	// Closed here rather than deferred, because the next thing that happens is
	// Excel opening this exact file. Windows would hand it a workbook still
	// held open by this process, and Excel would open it read-only — which is
	// the one state it must not be in.
	if err := f.Close(); err != nil {
		s.logger.Error("closing workbook", "path", dest, "error", err)
		writeError(w, http.StatusInternalServerError, "Could not finish writing the file.")
		return 0, false
	}
	return int64(written), true
}
