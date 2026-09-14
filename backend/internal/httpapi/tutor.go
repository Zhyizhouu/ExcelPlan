package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Zhyizhouu/excelplan/internal/datasets"
	"github.com/Zhyizhouu/excelplan/internal/tutor"
	"github.com/Zhyizhouu/excelplan/internal/vault"
)

// maxQuestion caps one message. Generous enough to paste a long macro, small
// enough that a runaway client cannot push a megabyte into the model.
const maxQuestion = 8000

// sampleRows is how much of the data the tutor is shown. Enough to see the
// shape and write an example against real values; not so much that the columns
// get lost in forty rows of it.
const sampleRows = 3

type tutorThreadResponse struct {
	Messages []tutor.Message `json:"messages"`
	// False when no API key is set. The tab then explains what to configure
	// rather than offering an input that cannot work.
	Configured bool `json:"configured"`
	Model      string `json:"model,omitempty"`
	// True when this case has a worked example, so the page can say whether
	// the tutor is reciting the intended answer or deriving one.
	HasExample bool `json:"hasExample"`
}

// handleTutorThread returns the conversation so far for one case.
func (s *Server) handleTutorThread(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	note, err := vault.FindBySlug(s.vaultRoot, slug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	_, hasExample := datasets.ExampleFor(note.Slug)

	writeJSON(w, http.StatusOK, tutorThreadResponse{
		Messages:   s.tutorLog.Thread(note.Slug),
		Configured: s.tutor.Configured(),
		Model:      s.tutor.Model(),
		HasExample: hasExample,
	})
}

type askRequest struct {
	Question string `json:"question"`
}

// handleTutorAsk sends one question, with the case attached, and stores both
// turns.
//
// The case context is rebuilt from the vault on every question rather than
// cached: the student may have edited the note in Obsidian between messages,
// and the whole premise of this app is that the file is what is true.
func (s *Server) handleTutorAsk(w http.ResponseWriter, r *http.Request) {
	if !s.tutor.Configured() {
		writeError(w, http.StatusServiceUnavailable,
			"No Gemini API key is set. Put one in backend/.env as GEMINI_API_KEY "+
				"and restart the server.")
		return
	}

	var body askRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Expected a JSON body with a `question` field.")
		return
	}
	question := strings.TrimSpace(body.Question)
	if question == "" {
		writeError(w, http.StatusBadRequest, "Ask something first.")
		return
	}
	if len(question) > maxQuestion {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("That is longer than %d characters. Trim it to the part you are stuck on.",
				maxQuestion))
		return
	}

	note, err := vault.FindBySlug(s.vaultRoot, r.PathValue("slug"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	history := s.tutorLog.Thread(note.Slug)
	blocks, err := s.tutor.Ask(
		r.Context(),
		tutor.SystemPrompt,
		history,
		s.caseContext(note).Render()+"\n\nTHE STUDENT ASKS\n"+question,
	)
	if err != nil {
		// The upstream message is shown as-is. It is the difference between a
		// bad key, a rate limit, and no internet, and the student is the only
		// person who can act on any of them.
		s.logger.Error("tutor", "slug", note.Slug, "error", err)
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	now := time.Now()
	if err := s.tutorLog.Append(note.Slug,
		tutor.Message{Role: tutor.RoleUser, Text: question, At: now},
		tutor.Message{Role: tutor.RoleModel, Blocks: blocks, At: time.Now()},
	); err != nil {
		// The answer is already correct and about to be shown; only the
		// transcript failed. Losing it is not worth losing the reply over.
		s.logger.Error("saving tutor thread", "slug", note.Slug, "error", err)
	}

	_, hasExample := datasets.ExampleFor(note.Slug)
	writeJSON(w, http.StatusOK, tutorThreadResponse{
		Messages:   s.tutorLog.Thread(note.Slug),
		Configured: true,
		Model:      s.tutor.Model(),
		HasExample: hasExample,
	})
}

// handleTutorClear forgets one case's conversation.
func (s *Server) handleTutorClear(w http.ResponseWriter, r *http.Request) {
	note, err := vault.FindBySlug(s.vaultRoot, r.PathValue("slug"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := s.tutorLog.Clear(note.Slug); err != nil {
		s.logger.Error("clearing tutor thread", "slug", note.Slug, "error", err)
		writeError(w, http.StatusInternalServerError, "Could not clear the conversation.")
		return
	}
	writeJSON(w, http.StatusOK, tutorThreadResponse{
		Messages:   nil,
		Configured: s.tutor.Configured(),
		Model:      s.tutor.Model(),
	})
}

// caseContext gathers everything the tutor is told about a case.
func (s *Server) caseContext(note vault.Note) tutor.CaseContext {
	ctx := tutor.CaseContext{
		Slug:  note.Slug,
		Title: note.Heading,
		Phase: note.Phase,
		Week:  note.Week,
	}
	if ctx.Title == "" {
		ctx.Title = note.Title
	}
	for _, f := range note.Fields {
		ctx.Fields = append(ctx.Fields, fmt.Sprintf("%s: %s", f.Label, f.Value))
	}

	example, hasExample := datasets.ExampleFor(note.Slug)
	if hasExample {
		ctx.ExpectedFormula = example.Formula
		ctx.ExpectedNote = example.ResultNote
		ctx.Requires = example.Requires
		ctx.Controls = example.Controls
	}

	// The dataset is only known through the example, since that is what names
	// which one a case uses. A case without an example gets its task and no
	// data, which the prompt already tells the tutor how to handle.
	if table, ok := datasets.Get(example.DatasetID); ok {
		narrowed := pick(table, example.InputColumns)
		ctx.DatasetName = narrowed.Name
		ctx.Columns = narrowed.Columns
		for i, row := range narrowed.Rows {
			if i >= sampleRows {
				break
			}
			cells := make([]string, 0, len(narrowed.Columns))
			for _, col := range narrowed.Columns {
				cells = append(cells, fmt.Sprintf("%v", row[col]))
			}
			ctx.SampleRows = append(ctx.SampleRows, strings.Join(cells, " | "))
		}
	}
	return ctx
}
