// Package main is the imprint-cli — a thin terminal client over the running
// imprint server. Subcommands hit the same HTTP endpoints the dashboard uses.
//
// Usage:
//
//	imprint-cli recall "what did I learn about JWT?"
//	imprint-cli search "drizzle migration"
//	imprint-cli status
//
// Configuration via env vars:
//
//	IMPRINT_URL    base URL (default http://localhost:3111)
//	IMPRINT_SECRET bearer token, if the server requires auth
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultBaseURL = "http://localhost:3111"
	defaultLimit   = 8
)

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(2)
	}

	switch os.Args[1] {
	case "recall":
		os.Exit(cmdRecall(os.Args[2:]))
	case "search":
		os.Exit(cmdSearch(os.Args[2:]))
	case "status":
		os.Exit(cmdStatus(os.Args[2:]))
	case "-h", "--help", "help":
		usage(os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "imprint-cli: unknown command %q\n\n", os.Args[1])
		usage(os.Stderr)
		os.Exit(2)
	}
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "imprint-cli — terminal client for the imprint server")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  recall <query>   ask the LLM, with citations")
	fmt.Fprintln(w, "  search <query>   raw hybrid search hits")
	fmt.Fprintln(w, "  status           pipeline counts and last-activity timestamps")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Env: IMPRINT_URL (default http://localhost:3111), IMPRINT_SECRET")
}

// --- recall -----------------------------------------------------------------

func cmdRecall(args []string) int {
	fs := flag.NewFlagSet("recall", flag.ExitOnError)
	limit := fs.Int("limit", defaultLimit, "max sources to retrieve")
	// Reorder so positionals can come before flags (`recall "query" --limit 4`).
	args = reorderFlagsFirst(args)
	fs.Parse(args)
	rest := fs.Args()
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "imprint-cli recall: missing query")
		return 2
	}
	query := strings.Join(rest, " ")

	var resp struct {
		Answer  string `json:"answer"`
		Sources []struct {
			ID    string  `json:"id"`
			Type  string  `json:"type"`
			Title string  `json:"title"`
			Score float64 `json:"score"`
		} `json:"sources"`
		Used    int    `json:"used"`
		Skipped string `json:"skipped"`
	}
	if err := post("/imprint/recall", map[string]any{"query": query, "limit": *limit}, &resp); err != nil {
		fmt.Fprintf(os.Stderr, "recall: %v\n", err)
		return 1
	}

	if resp.Answer != "" {
		fmt.Println(resp.Answer)
	}
	if resp.Skipped != "" {
		fmt.Fprintf(os.Stderr, "\n[skipped: %s]\n", resp.Skipped)
	}
	if len(resp.Sources) > 0 {
		fmt.Println()
		fmt.Println("Sources:")
		for i, s := range resp.Sources {
			fmt.Printf("  [%d] (%s) %s\n", i+1, s.Type, s.Title)
		}
	}
	return 0
}

// --- search -----------------------------------------------------------------

func cmdSearch(args []string) int {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	limit := fs.Int("limit", 20, "max results")
	args = reorderFlagsFirst(args)
	fs.Parse(args)
	rest := fs.Args()
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "imprint-cli search: missing query")
		return 2
	}
	query := strings.Join(rest, " ")

	var resp struct {
		Count   int `json:"count"`
		Results []struct {
			Title    string  `json:"title"`
			Type     string  `json:"type"`
			Score    float64 `json:"score"`
			Concepts []string `json:"concepts"`
		} `json:"results"`
	}
	if err := post("/imprint/search", map[string]any{"query": query, "limit": *limit}, &resp); err != nil {
		fmt.Fprintf(os.Stderr, "search: %v\n", err)
		return 1
	}

	if len(resp.Results) == 0 {
		fmt.Println("(no matches)")
		return 0
	}
	for i, r := range resp.Results {
		fmt.Printf("%2d. [%s] %s\n", i+1, r.Type, r.Title)
		if len(r.Concepts) > 0 {
			fmt.Printf("    concepts: %s\n", strings.Join(r.Concepts, ", "))
		}
	}
	return 0
}

// --- status -----------------------------------------------------------------

func cmdStatus(args []string) int {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	fs.Parse(args)

	var stats struct {
		RawCount        int                       `json:"rawCount"`
		CompressedCount int                       `json:"compressedCount"`
		MemoryCount     int                       `json:"memoryCount"`
		LessonCount     int                       `json:"lessonCount"`
		InsightCount    int                       `json:"insightCount"`
		ActiveSessions  int                       `json:"activeSessions"`
		Backlog         int                       `json:"backlog"`
		LastByAction    map[string]any            `json:"lastByAction"`
		Usage           struct {
			Calls        int64 `json:"calls"`
			Failures     int64 `json:"failures"`
			PromptTokens int64 `json:"promptTokens"`
			OutputTokens int64 `json:"outputTokens"`
		} `json:"usage"`
	}
	if err := get("/imprint/pipeline/status", &stats); err != nil {
		fmt.Fprintf(os.Stderr, "status: %v\n", err)
		return 1
	}

	fmt.Printf("raw observations  %d\n", stats.RawCount)
	fmt.Printf("compressed        %d\n", stats.CompressedCount)
	if stats.Backlog > 0 {
		fmt.Printf("backlog           %d  (raw without compressed)\n", stats.Backlog)
	}
	fmt.Printf("memories          %d\n", stats.MemoryCount)
	fmt.Printf("lessons           %d\n", stats.LessonCount)
	fmt.Printf("insights          %d\n", stats.InsightCount)
	fmt.Printf("active sessions   %d\n", stats.ActiveSessions)

	if len(stats.LastByAction) > 0 {
		fmt.Println()
		fmt.Println("Last activity:")
		for k, v := range stats.LastByAction {
			ts, _ := v.(string)
			if ts == "" {
				continue
			}
			fmt.Printf("  %-22s %s\n", k, ts)
		}
	}

	if stats.Usage.Calls > 0 {
		fmt.Println()
		fmt.Printf("LLM usage (since boot): %d calls, %d prompt + %d output tokens",
			stats.Usage.Calls, stats.Usage.PromptTokens, stats.Usage.OutputTokens)
		if stats.Usage.Failures > 0 {
			fmt.Printf(", %d failures", stats.Usage.Failures)
		}
		fmt.Println()
	}
	return 0
}

// reorderFlagsFirst pulls -flag and --flag tokens (and their values, when the
// flag is non-boolean) to the front of the slice so flag.Parse can handle
// `recall "query" --limit 4` as easily as `recall --limit 4 "query"`.
// We treat any token starting with `-` as a flag; the immediate next token
// is consumed only when it doesn't itself start with `-` and the flag uses
// an explicit value.
func reorderFlagsFirst(args []string) []string {
	flags := []string{}
	rest := []string{}
	i := 0
	for i < len(args) {
		a := args[i]
		if a == "--" {
			rest = append(rest, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
			// If the flag isn't of the form --x=val and the next token isn't a flag,
			// assume it's the flag's value and pull it along.
			if !strings.Contains(a, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flags = append(flags, args[i+1])
				i += 2
				continue
			}
			i++
			continue
		}
		rest = append(rest, a)
		i++
	}
	return append(flags, rest...)
}

// --- HTTP helpers -----------------------------------------------------------

var client = &http.Client{Timeout: 60 * time.Second}

func baseURL() string {
	if v := os.Getenv("IMPRINT_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultBaseURL
}

func get(path string, out any) error {
	req, err := http.NewRequest("GET", baseURL()+path, nil)
	if err != nil {
		return err
	}
	return doRequest(req, out)
}

func post(path string, body any, out any) error {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", baseURL()+path, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return doRequest(req, out)
}

func doRequest(req *http.Request, out any) error {
	if secret := os.Getenv("IMPRINT_SECRET"); secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		var errResp struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(body, &errResp)
		if errResp.Error != "" {
			return fmt.Errorf("server %d: %s", resp.StatusCode, errResp.Error)
		}
		return fmt.Errorf("server %d: %s", resp.StatusCode, string(body))
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}
	}
	return nil
}
