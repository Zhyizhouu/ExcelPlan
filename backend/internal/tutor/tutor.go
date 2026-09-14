// Package tutor answers a student's questions about the case they are on.
//
// It talks to Gemini, but the interesting part is not the API call — it is
// what gets sent with every question. The app already knows which case is open,
// what it asks for, which columns the data has, and (for the ten cases that
// have a worked example) the intended formula and result. All of that is
// assembled into the request, so a beginner can type "why doesn't this work"
// and get a useful answer without first explaining their own homework.
//
// The reply comes back as typed blocks rather than prose. Asking for a schema
// costs nothing at the API and buys two things: tables render with the same
// component the case pages use, and the model cannot emit layout the frontend
// was never designed for.
package tutor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const endpoint = "https://generativelanguage.googleapis.com/v1beta/models"

// Role is who said a thing. Gemini's own vocabulary: "model", not "assistant".
type Role string

const (
	RoleUser  Role = "user"
	RoleModel Role = "model"
)

// Block is one piece of an answer. The type field decides which of the rest
// are populated, which keeps the wire format flat enough for a JSON schema
// while still letting the page render a table as a table.
type Block struct {
	// One of: text, table, code, steps.
	Type string `json:"type"`
	// text
	Text string `json:"text,omitempty"`
	// table
	Columns []string   `json:"columns,omitempty"`
	Rows    [][]string `json:"rows,omitempty"`
	// code
	Language string `json:"language,omitempty"`
	Code     string `json:"code,omitempty"`
	// steps
	Items []string `json:"items,omitempty"`
}

// Message is one turn of a thread, as stored and as sent back to the browser.
type Message struct {
	Role Role `json:"role"`
	// What the student typed. Empty on a model turn.
	Text string `json:"text,omitempty"`
	// The model's structured answer. Nil on a student turn.
	Blocks []Block   `json:"blocks,omitempty"`
	At     time.Time `json:"at"`
}

type Client struct {
	apiKey string
	model  string
	http   *http.Client
}

// New returns a client, or nil when no key is configured.
//
// A nil client is a first-class state rather than an error: the server must
// start and serve the whole plan without a Gemini key, since the tutor is one
// tab and the rest of the app has nothing to do with it. Callers ask Configured.
func New(apiKey, model string) *Client {
	if strings.TrimSpace(apiKey) == "" {
		return nil
	}
	if model == "" {
		model = "gemini-flash-latest"
	}
	return &Client{
		apiKey: apiKey,
		model:  model,
		// Generous: a long answer with tables takes a while, and the student is
		// watching a "thinking" line rather than a frozen page.
		http: &http.Client{Timeout: 90 * time.Second},
	}
}

// Configured reports whether a key was supplied. Safe on a nil client.
func (c *Client) Configured() bool { return c != nil }

// Model is the id in use, for the frontend to show in the tutor's footer.
func (c *Client) Model() string {
	if c == nil {
		return ""
	}
	return c.model
}

// --- the wire format -------------------------------------------------------

type part struct {
	Text string `json:"text"`
}

type content struct {
	Role  Role   `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

type generationConfig struct {
	Temperature      float64 `json:"temperature"`
	ResponseMimeType string  `json:"responseMimeType"`
	ResponseSchema   any     `json:"responseSchema"`
}

type generateRequest struct {
	SystemInstruction *content         `json:"system_instruction,omitempty"`
	Contents          []content        `json:"contents"`
	GenerationConfig  generationConfig `json:"generationConfig"`
}

type generateResponse struct {
	Candidates []struct {
		Content      content `json:"content"`
		FinishReason string  `json:"finishReason"`
	} `json:"candidates"`
	PromptFeedback struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

// answerSchema pins the shape of a reply.
//
// Every field but Type is optional, because a text block has no columns and a
// table has no code. Requiring them all would force the model to invent empty
// values for fields the block does not use, and it reliably fills such fields
// with plausible nonsense rather than leaving them blank.
var answerSchema = map[string]any{
	"type": "OBJECT",
	"properties": map[string]any{
		"blocks": map[string]any{
			"type": "ARRAY",
			"items": map[string]any{
				"type": "OBJECT",
				"properties": map[string]any{
					"type": map[string]any{
						"type": "STRING",
						"enum": []string{"text", "table", "code", "steps"},
					},
					"text":     map[string]any{"type": "STRING"},
					"columns":  map[string]any{"type": "ARRAY", "items": map[string]any{"type": "STRING"}},
					"rows":     map[string]any{"type": "ARRAY", "items": map[string]any{"type": "ARRAY", "items": map[string]any{"type": "STRING"}}},
					"language": map[string]any{"type": "STRING"},
					"code":     map[string]any{"type": "STRING"},
					"items":    map[string]any{"type": "ARRAY", "items": map[string]any{"type": "STRING"}},
				},
				"required": []string{"type"},
			},
		},
	},
	"required": []string{"blocks"},
}

// Ask sends the thread plus one new question and returns the answer's blocks.
//
// History is replayed in full on every call — Gemini keeps no session state, so
// the conversation only exists because this sends it. A model turn goes back as
// the JSON it produced, which is what it said and therefore what it should see
// itself having said.
func (c *Client) Ask(
	ctx context.Context,
	systemPrompt string,
	history []Message,
	question string,
) ([]Block, error) {
	if c == nil {
		return nil, fmt.Errorf("tutor is not configured")
	}

	contents := make([]content, 0, len(history)+1)
	for _, m := range history {
		text := m.Text
		if m.Role == RoleModel {
			encoded, err := json.Marshal(map[string]any{"blocks": m.Blocks})
			if err != nil {
				continue // a turn we cannot re-encode is better dropped than fatal
			}
			text = string(encoded)
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		contents = append(contents, content{Role: m.Role, Parts: []part{{Text: text}}})
	}
	contents = append(contents, content{Role: RoleUser, Parts: []part{{Text: question}}})

	body, err := json.Marshal(generateRequest{
		SystemInstruction: &content{Parts: []part{{Text: systemPrompt}}},
		Contents:          contents,
		GenerationConfig: generationConfig{
			// Low, not zero. The same question asked twice should get the same
			// advice; a tutor that contradicts itself stops being trusted. Not
			// zero, because an explanation reworded on a second attempt is
			// often what an explanation needed.
			Temperature:      0.3,
			ResponseMimeType: "application/json",
			ResponseSchema:   answerSchema,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	url := fmt.Sprintf("%s/%s:generateContent", endpoint, c.model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// In a header rather than a query string: a key in a URL turns up in
	// proxy logs and error messages that get pasted into issues.
	req.Header.Set("x-goog-api-key", c.apiKey)

	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reaching Gemini: %w", err)
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("reading Gemini's reply: %w", err)
	}

	var parsed generateResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("Gemini sent something that is not JSON (HTTP %d)", res.StatusCode)
	}
	// Overload before anything else. The newest flash models routinely answer
	// 503 at busy times, and the upstream text for it ("This model is currently
	// experiencing high demand") reads like something is wrong with the
	// question rather than with the queue. Nothing is broken and nothing needs
	// changing — the answer is to ask again.
	if res.StatusCode == http.StatusTooManyRequests ||
		res.StatusCode == http.StatusServiceUnavailable {
		return nil, fmt.Errorf(
			"Gemini is busy right now (%s is in high demand). Ask again in a moment, "+
				"or set GEMINI_MODEL in backend/.env to a less busy model.", c.model)
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("Gemini refused: %s", parsed.Error.Message)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Gemini answered %d", res.StatusCode)
	}
	if parsed.PromptFeedback.BlockReason != "" {
		return nil, fmt.Errorf("Gemini blocked the question (%s)", parsed.PromptFeedback.BlockReason)
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("Gemini returned an empty answer")
	}

	var answer struct {
		Blocks []Block `json:"blocks"`
	}
	text := parsed.Candidates[0].Content.Parts[0].Text
	if err := json.Unmarshal([]byte(text), &answer); err != nil {
		// The schema makes this unlikely, but a truncated reply is still
		// possible — better a plain paragraph than an error page.
		return []Block{{Type: "text", Text: text}}, nil
	}
	if len(answer.Blocks) == 0 {
		return nil, fmt.Errorf("Gemini returned an answer with nothing in it")
	}
	return answer.Blocks, nil
}
