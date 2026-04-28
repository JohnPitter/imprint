package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"

	"imprint/internal/privacy"
	"imprint/internal/store"
	"imprint/internal/types"

	"github.com/google/uuid"
)

// CompressSubmitter is satisfied by pipeline.Worker to avoid a direct import cycle.
type CompressSubmitter interface {
	Submit(raw *store.RawObservationRow)
}

// ObserveService processes incoming hook payloads and stores raw observations.
type ObserveService struct {
	c             *Container
	maxPerSession int
	maxToolOutput int
	compressor    CompressSubmitter // optional, set via SetCompressor
}

// NewObserveService creates a new ObserveService with the given limits.
func NewObserveService(c *Container, maxPerSession, maxToolOutput int) *ObserveService {
	return &ObserveService{
		c:             c,
		maxPerSession: maxPerSession,
		maxToolOutput: maxToolOutput,
	}
}

// SetCompressor attaches a background compression worker.
func (s *ObserveService) SetCompressor(w CompressSubmitter) {
	s.compressor = w
}

// Observe processes an incoming hook payload and stores the raw observation.
// Returns nil, nil when the observation is a duplicate or the session rate limit is exceeded.
func (s *ObserveService) Observe(payload *types.HookPayload) (*store.RawObservationRow, error) {
	if payload.SessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	// Check rate limit: max observations per session.
	count, err := s.c.Observations.CountBySession(payload.SessionID)
	if err != nil {
		return nil, fmt.Errorf("check observation count: %w", err)
	}
	if count >= s.maxPerSession {
		return nil, nil // silently skip
	}

	// Truncate tool output if it exceeds maxToolOutput.
	toolOutput := truncateRawJSON(payload.ToolOutput, s.maxToolOutput)

	// Strip private data from string fields.
	toolInput := scrubRawJSON(payload.ToolInput)
	toolOutput = scrubRawJSON(toolOutput)
	var userPrompt *string
	if payload.UserPrompt != nil {
		scrubbed := privacy.StripPrivateData(*payload.UserPrompt)
		userPrompt = &scrubbed
	} else if payload.Prompt != nil {
		scrubbed := privacy.StripPrivateData(*payload.Prompt)
		userPrompt = &scrubbed
	}

	// Dedup check: SHA-256 of sessionID + hookType + toolName + truncated input.
	dedupInput := payload.SessionID + string(payload.HookType)
	if payload.ToolName != nil {
		dedupInput += *payload.ToolName
	}
	if len(toolInput) > 0 {
		dedupInput += string(toolInput)
	}
	hash := sha256.Sum256([]byte(dedupInput))
	hashHex := hex.EncodeToString(hash[:])

	isNew, err := s.c.Observations.InsertDedup(hashHex)
	if err != nil {
		return nil, fmt.Errorf("dedup check: %w", err)
	}
	if !isNew {
		return nil, nil // duplicate, skip
	}

	// Build raw JSON payload for the raw column.
	rawBytes, _ := json.Marshal(payload)
	rawJSON := json.RawMessage(privacy.StripPrivateData(string(rawBytes)))

	obsID := "obs_" + uuid.New().String()[:8]
	obs := &store.RawObservationRow{
		ID:         obsID,
		SessionID:  payload.SessionID,
		Timestamp:  payload.Timestamp,
		HookType:   string(payload.HookType),
		ToolName:   payload.ToolName,
		ToolInput:  toolInput,
		ToolOutput: toolOutput,
		UserPrompt: userPrompt,
		Raw:        rawJSON,
	}

	if obs.Timestamp.IsZero() {
		obs.Timestamp = time.Now()
	}

	if err := s.c.Observations.CreateRaw(obs); err != nil {
		return nil, fmt.Errorf("store observation: %w", err)
	}

	toolName := ""
	if obs.ToolName != nil {
		toolName = *obs.ToolName
	}
	s.c.LogAudit("observation.create", obsID, "observation", map[string]any{"tool": toolName, "hook": string(payload.HookType)})

	// Increment session observation count.
	if err := s.c.Sessions.IncrementObservationCount(payload.SessionID); err != nil {
		// Non-fatal: log but do not fail the observation.
		_ = err
	}

	// Submit for background LLM compression if worker is attached.
	// Only compress observations that have meaningful content (tool use with input/output).
	if s.compressor != nil && obs.ToolName != nil && *obs.ToolName != "" {
		s.compressor.Submit(obs)
	}

	return obs, nil
}

// ListRaw returns raw observations for a session with pagination.
func (s *ObserveService) ListRaw(sessionID string, limit, offset int) ([]store.RawObservationRow, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return s.c.Observations.ListRaw(sessionID, limit, offset)
}

// ListCompressed returns compressed observations for a session with pagination.
func (s *ObserveService) ListCompressed(sessionID string, limit, offset int) ([]store.CompressedObservationRow, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return s.c.Observations.ListCompressed(sessionID, limit, offset)
}

// truncateRawJSON returns a JSON raw message no longer than maxLen bytes.
// If the raw message is a JSON string, it is decoded, the inner string is
// truncated on a UTF-8 rune boundary, and re-encoded so the result stays
// valid JSON. For non-string JSON values (objects/arrays/numbers/etc) we
// fall back to a JSON-encoded marker string instead of slicing bytes,
// since cutting JSON mid-token would produce invalid JSON.
func truncateRawJSON(raw json.RawMessage, maxLen int) json.RawMessage {
	if len(raw) <= maxLen {
		return raw
	}
	// Try to decode as a JSON string and truncate the contents.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		// JSON string: budget for the surrounding quotes.
		inner := maxLen - 2
		if inner < 0 {
			inner = 0
		}
		if len(s) > inner {
			i := inner
			for i > 0 && !utf8.RuneStart(s[i]) {
				i--
			}
			s = s[:i]
		}
		out, _ := json.Marshal(s)
		return out
	}
	// Non-string JSON: replace with a marker so we never store invalid JSON.
	out, _ := json.Marshal(fmt.Sprintf("[truncated %d bytes]", len(raw)))
	return out
}

// scrubRawJSON strips private data from a JSON raw message.
func scrubRawJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	return json.RawMessage(privacy.StripPrivateData(string(raw)))
}
