package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Codex ChatGPT-OAuth constants, mirrored from the official Codex CLI
// (openai/codex codex-rs): the public OAuth client id, the token refresh
// endpoint, and the internal ChatGPT "codex" Responses endpoint that bills
// against the user's ChatGPT subscription. Using these lets Imprint reuse an
// existing `codex login` (ChatGPT plan) for background work — no API key — the
// way the Claude Code path reuses the Claude OAuth token.
const (
	codexClientID   = "app_EMoamEEZ73f0CkXaXp7hrann"
	codexOriginator = "codex_cli_rs"
)

// Endpoint URLs are vars (not consts) so tests can point them at mock servers.
var (
	codexTokenURL     = "https://auth.openai.com/oauth/token"
	codexResponsesURL = "https://chatgpt.com/backend-api/codex/responses"
)

// CodexOAuthProvider calls the ChatGPT-backend Responses API using the OAuth
// tokens stored by `codex login` in ~/.codex/auth.json. It refreshes the access
// token when expired (writing the rotated token back without clobbering Codex's
// other auth fields) and parses the streamed Responses API output.
type CodexOAuthProvider struct {
	authPath        string
	model           string
	reasoningEffort string
	client          *http.Client

	mu           sync.Mutex
	accessToken  string
	refreshToken string
	idToken      string
	accountID    string
	exp          time.Time
	loaded       bool
}

// NewCodexOAuthProvider builds the provider. authPath defaults to
// ~/.codex/auth.json; model defaults to "gpt-5".
func NewCodexOAuthProvider(authPath, model, reasoningEffort string) *CodexOAuthProvider {
	if authPath == "" {
		if home, err := os.UserHomeDir(); err == nil {
			authPath = filepath.Join(home, ".codex", "auth.json")
		}
	}
	if model == "" {
		model = "gpt-5"
	}
	return &CodexOAuthProvider{
		authPath:        authPath,
		model:           model,
		reasoningEffort: reasoningEffort,
		client:          &http.Client{Timeout: 120 * time.Second},
	}
}

func (p *CodexOAuthProvider) Name() string { return "codex-oauth" }

// Available reports whether a usable ChatGPT-OAuth credential exists. It loads
// auth.json lazily; api-key-only files (no tokens) are not handled here.
func (p *CodexOAuthProvider) Available() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.loaded {
		_ = p.loadLocked()
	}
	return p.accessToken != "" || p.refreshToken != ""
}

// codexAuthFile is the subset of ~/.codex/auth.json we read. Unknown fields are
// preserved separately on write (see persistTokensLocked).
type codexAuthFile struct {
	Tokens *struct {
		IDToken      string `json:"id_token"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		AccountID    string `json:"account_id"`
	} `json:"tokens"`
}

// loadLocked reads tokens from auth.json into memory. Caller holds p.mu.
func (p *CodexOAuthProvider) loadLocked() error {
	p.loaded = true
	data, err := os.ReadFile(p.authPath)
	if err != nil {
		return err
	}
	var af codexAuthFile
	if err := json.Unmarshal(data, &af); err != nil {
		return err
	}
	if af.Tokens == nil {
		return fmt.Errorf("codex auth.json has no ChatGPT tokens (api-key login?)")
	}
	p.accessToken = af.Tokens.AccessToken
	p.refreshToken = af.Tokens.RefreshToken
	p.idToken = af.Tokens.IDToken
	p.accountID = af.Tokens.AccountID
	if p.accountID == "" {
		p.accountID = accountIDFromJWT(p.idToken)
	}
	p.exp = jwtExpiry(p.accessToken)
	return nil
}

// ensureFreshLocked refreshes the access token if missing or within 60s of
// expiry. Caller holds p.mu.
func (p *CodexOAuthProvider) ensureFreshLocked(ctx context.Context) error {
	if !p.loaded {
		if err := p.loadLocked(); err != nil {
			return err
		}
	}
	if p.accessToken != "" && time.Until(p.exp) > 60*time.Second {
		return nil
	}
	if p.refreshToken == "" {
		return fmt.Errorf("codex-oauth: no refresh token available")
	}
	return p.refreshLocked(ctx)
}

type codexRefreshResp struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
}

// refreshLocked exchanges the refresh token for a new access token and persists
// any rotated tokens back to auth.json. Caller holds p.mu.
func (p *CodexOAuthProvider) refreshLocked(ctx context.Context) error {
	body, _ := json.Marshal(map[string]string{
		"client_id":     codexClientID,
		"grant_type":    "refresh_token",
		"refresh_token": p.refreshToken,
		"scope":         "openid profile email",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, codexTokenURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("codex-oauth: refresh request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("codex-oauth: refresh HTTP %d", resp.StatusCode)
	}
	var rr codexRefreshResp
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return fmt.Errorf("codex-oauth: refresh decode: %w", err)
	}
	if rr.AccessToken == "" {
		return fmt.Errorf("codex-oauth: refresh returned no access token")
	}
	p.accessToken = rr.AccessToken
	p.exp = jwtExpiry(rr.AccessToken)
	if rr.IDToken != "" {
		p.idToken = rr.IDToken
		if id := accountIDFromJWT(rr.IDToken); id != "" {
			p.accountID = id
		}
	}
	if rr.RefreshToken != "" && rr.RefreshToken != p.refreshToken {
		p.refreshToken = rr.RefreshToken
	}
	// Persist back so Codex keeps working if the refresh token rotated.
	p.persistTokensLocked()
	return nil
}

// persistTokensLocked writes the current tokens back to auth.json, preserving
// every other field Codex stores (auth_mode, OPENAI_API_KEY, etc.) by editing a
// generic map. Best-effort: a write failure never blocks the call.
func (p *CodexOAuthProvider) persistTokensLocked() {
	data, err := os.ReadFile(p.authPath)
	if err != nil {
		return
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return
	}
	tokens := map[string]any{
		"id_token":      p.idToken,
		"access_token":  p.accessToken,
		"refresh_token": p.refreshToken,
		"account_id":    p.accountID,
	}
	tb, _ := json.Marshal(tokens)
	root["tokens"] = tb
	if lr, err := json.Marshal(time.Now().UTC().Format(time.RFC3339)); err == nil {
		root["last_refresh"] = lr
	}
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return
	}
	tmp := p.authPath + ".imprint.tmp"
	if os.WriteFile(tmp, out, 0o600) == nil {
		_ = os.Rename(tmp, p.authPath)
	}
}

// ── Responses API ────────────────────────────────────────────────────────────

type responsesRequest struct {
	Model        string           `json:"model"`
	Instructions string           `json:"instructions,omitempty"`
	Input        []responsesInput `json:"input"`
	Stream       bool             `json:"stream"`
	Store        bool             `json:"store"`
	Reasoning    *responsesReason `json:"reasoning,omitempty"`
}

type responsesReason struct {
	Effort string `json:"effort"`
}

type responsesInput struct {
	Type    string             `json:"type"`
	Role    string             `json:"role"`
	Content []responsesContent `json:"content"`
}

type responsesContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (p *CodexOAuthProvider) Complete(ctx context.Context, req CompletionRequest) (string, error) {
	p.mu.Lock()
	if err := p.ensureFreshLocked(ctx); err != nil {
		p.mu.Unlock()
		return "", err
	}
	access, account := p.accessToken, p.accountID
	p.mu.Unlock()

	body := responsesRequest{
		Model:        p.model,
		Instructions: req.SystemPrompt,
		Input: []responsesInput{{
			Type:    "message",
			Role:    "user",
			Content: []responsesContent{{Type: "input_text", Text: req.UserPrompt}},
		}},
		Stream: true, // the ChatGPT codex backend only supports streaming
		Store:  false,
	}
	if p.reasoningEffort != "" {
		body.Reasoning = &responsesReason{Effort: p.reasoningEffort}
	}

	payload, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, codexResponsesURL, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Authorization", "Bearer "+access)
	if account != "" {
		httpReq.Header.Set("chatgpt-account-id", account)
	}
	httpReq.Header.Set("OpenAI-Beta", "responses=experimental")
	httpReq.Header.Set("originator", codexOriginator)
	httpReq.Header.Set("session_id", uuid.New().String())
	httpReq.Header.Set("User-Agent", codexOriginator+" (imprint)")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("codex-oauth: request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(resp.Body)
		return "", fmt.Errorf("codex-oauth: HTTP %d: %s", resp.StatusCode, truncateForErr(buf.String()))
	}

	text, inTok, outTok, err := parseResponsesSSE(resp.Body)
	if err != nil {
		return "", err
	}
	if inTok > 0 || outTok > 0 {
		GlobalBudget.Record(req.SessionID, inTok+outTok)
		if sink := SpendSink; sink != nil && req.SpendPoint != "" {
			sink(SpendEvent{Provider: "codex-oauth", SpendPoint: req.SpendPoint, SessionID: req.SessionID, Project: req.Project, InputTokens: inTok, OutputTokens: outTok})
		}
	}
	GlobalUsage.Record("codex-oauth", inTok, outTok, false)
	return text, nil
}

// parseResponsesSSE consumes the Responses API event stream, accumulating output
// text deltas and reading token usage from the terminal response.completed event.
func parseResponsesSSE(r interface{ Read([]byte) (int, error) }) (string, int, int, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var textB strings.Builder
	var inTok, outTok int
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var ev struct {
			Type     string `json:"type"`
			Delta    string `json:"delta"`
			Response struct {
				Usage struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
				Output []struct {
					Content []struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"content"`
				} `json:"output"`
			} `json:"response"`
		}
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			continue
		}
		switch ev.Type {
		case "response.output_text.delta":
			textB.WriteString(ev.Delta)
		case "response.completed":
			inTok = ev.Response.Usage.InputTokens
			outTok = ev.Response.Usage.OutputTokens
			if textB.Len() == 0 { // some backends only send the full text here
				for _, o := range ev.Response.Output {
					for _, c := range o.Content {
						if c.Type == "output_text" {
							textB.WriteString(c.Text)
						}
					}
				}
			}
		case "response.failed", "error":
			return "", inTok, outTok, fmt.Errorf("codex-oauth: stream reported failure")
		}
	}
	if err := sc.Err(); err != nil {
		return "", inTok, outTok, fmt.Errorf("codex-oauth: read stream: %w", err)
	}
	return textB.String(), inTok, outTok, nil
}

// ── JWT helpers ──────────────────────────────────────────────────────────────

// accountIDFromJWT extracts chatgpt_account_id from an id_token's
// "https://api.openai.com/auth" claim. Returns "" on any failure.
func accountIDFromJWT(jwt string) string {
	claims := jwtClaims(jwt)
	if claims == nil {
		return ""
	}
	if auth, ok := claims["https://api.openai.com/auth"].(map[string]any); ok {
		if id, ok := auth["chatgpt_account_id"].(string); ok {
			return id
		}
	}
	return ""
}

// jwtExpiry reads the "exp" claim. Returns zero time if absent (treated as expired).
func jwtExpiry(jwt string) time.Time {
	claims := jwtClaims(jwt)
	if claims == nil {
		return time.Time{}
	}
	if exp, ok := claims["exp"].(float64); ok {
		return time.Unix(int64(exp), 0)
	}
	return time.Time{}
}

func jwtClaims(jwt string) map[string]any {
	parts := strings.Split(jwt, ".")
	if len(parts) < 2 {
		return nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Some tokens pad; try standard URL encoding.
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return nil
		}
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil
	}
	return claims
}

func truncateForErr(s string) string {
	if len(s) > 300 {
		return s[:300] + "…"
	}
	return s
}
