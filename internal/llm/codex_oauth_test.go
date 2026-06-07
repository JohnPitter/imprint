package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func makeJWT(claims map[string]any) string {
	h := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	pb, _ := json.Marshal(claims)
	return h + "." + base64.RawURLEncoding.EncodeToString(pb) + ".sig"
}

func writeAuthJSON(t *testing.T, dir, access, refresh, idToken string) string {
	t.Helper()
	auth := map[string]any{
		"auth_mode":      "Chatgpt",
		"OPENAI_API_KEY": nil,
		"tokens": map[string]any{
			"id_token": idToken, "access_token": access, "refresh_token": refresh, "account_id": "",
		},
		"last_refresh": "2026-01-01T00:00:00Z",
	}
	b, _ := json.MarshalIndent(auth, "", "  ")
	p := filepath.Join(dir, "auth.json")
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCodexOAuth_AccountIDFromJWT(t *testing.T) {
	jwt := makeJWT(map[string]any{
		"https://api.openai.com/auth": map[string]any{"chatgpt_account_id": "acc_xyz"},
	})
	if got := accountIDFromJWT(jwt); got != "acc_xyz" {
		t.Errorf("accountIDFromJWT = %q, want acc_xyz", got)
	}
}

func TestCodexOAuth_CompleteParsesSSE(t *testing.T) {
	var gotAuth, gotAccount, gotOriginator, gotBeta string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccount = r.Header.Get("chatgpt-account-id")
		gotOriginator = r.Header.Get("originator")
		gotBeta = r.Header.Get("OpenAI-Beta")
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"Hello \"}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"world\"}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":11,\"output_tokens\":2}}}\n\n")
	}))
	defer srv.Close()
	old := codexResponsesURL
	codexResponsesURL = srv.URL
	defer func() { codexResponsesURL = old }()

	dir := t.TempDir()
	access := makeJWT(map[string]any{"exp": float64(time.Now().Add(time.Hour).Unix())})
	id := makeJWT(map[string]any{"https://api.openai.com/auth": map[string]any{"chatgpt_account_id": "acc_1"}})
	authPath := writeAuthJSON(t, dir, access, "rt", id)

	p := NewCodexOAuthProvider(authPath, "gpt-5", "minimal")
	if !p.Available() {
		t.Fatal("expected available with tokens")
	}
	out, err := p.Complete(context.Background(), CompletionRequest{SystemPrompt: "sys", UserPrompt: "hi", SpendPoint: "compress"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if out != "Hello world" {
		t.Errorf("output = %q, want 'Hello world'", out)
	}
	if gotAuth != "Bearer "+access {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotAccount != "acc_1" {
		t.Errorf("chatgpt-account-id = %q, want acc_1", gotAccount)
	}
	if gotOriginator != "codex_cli_rs" {
		t.Errorf("originator = %q", gotOriginator)
	}
	if gotBeta != "responses=experimental" {
		t.Errorf("OpenAI-Beta = %q", gotBeta)
	}
}

func TestCodexOAuth_RefreshesExpiredToken(t *testing.T) {
	// Token endpoint returns a fresh access token.
	newAccess := makeJWT(map[string]any{"exp": float64(time.Now().Add(time.Hour).Unix())})
	tok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["grant_type"] != "refresh_token" || body["client_id"] != codexClientID {
			t.Errorf("bad refresh body: %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"access_token":%q,"refresh_token":"rt-2"}`, newAccess)
	}))
	defer tok.Close()
	var bearerSeen string
	resp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bearerSeen = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\n")
	}))
	defer resp.Close()
	ot, or := codexTokenURL, codexResponsesURL
	codexTokenURL, codexResponsesURL = tok.URL, resp.URL
	defer func() { codexTokenURL, codexResponsesURL = ot, or }()

	dir := t.TempDir()
	expired := makeJWT(map[string]any{"exp": float64(time.Now().Add(-time.Hour).Unix())}) // already expired
	authPath := writeAuthJSON(t, dir, expired, "rt-1", "")

	p := NewCodexOAuthProvider(authPath, "gpt-5", "")
	out, err := p.Complete(context.Background(), CompletionRequest{UserPrompt: "hi"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if out != "ok" {
		t.Errorf("output = %q", out)
	}
	if bearerSeen != "Bearer "+newAccess {
		t.Errorf("expected refreshed bearer, got %q", bearerSeen)
	}
	// auth.json rewritten with the new tokens, preserving auth_mode.
	data, _ := os.ReadFile(authPath)
	var root map[string]any
	_ = json.Unmarshal(data, &root)
	if root["auth_mode"] != "Chatgpt" {
		t.Errorf("auth_mode not preserved: %v", root["auth_mode"])
	}
	toks, _ := root["tokens"].(map[string]any)
	if toks["access_token"] != newAccess || toks["refresh_token"] != "rt-2" {
		t.Errorf("tokens not persisted: %+v", toks)
	}
}
