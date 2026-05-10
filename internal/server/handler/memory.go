package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"imprint/internal/llm"
	"imprint/internal/service"
	"imprint/internal/types"
)

// MemoryHandler holds HTTP handlers for memory endpoints.
type MemoryHandler struct {
	svc *service.RememberService
	llm llm.LLMProvider // opcional: se nil, cluster summarizer responde 503
	// Cache de cluster summary keyed pela "fingerprint" (sorted ids
	// concatenados). Lifetime do processo — barato e útil pra hover
	// repetido na mesma comunidade.
	clusterCache   map[string]string
	clusterCacheMu sync.RWMutex
}

// NewMemoryHandler creates a new MemoryHandler.
// llmProvider é opcional; quando nil, endpoints LLM-dependent retornam 503.
func NewMemoryHandler(svc *service.RememberService, llmProvider llm.LLMProvider) *MemoryHandler {
	return &MemoryHandler{
		svc:          svc,
		llm:          llmProvider,
		clusterCache: make(map[string]string),
	}
}

// HandleRemember handles POST /imprint/remember.
func (h *MemoryHandler) HandleRemember(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type     types.MemoryType `json:"type"`
		Title    string           `json:"title"`
		Content  string           `json:"content"`
		Concepts []string         `json:"concepts"`
		Files    []string         `json:"files"`
		Strength int              `json:"strength"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	mem, err := h.svc.Remember(req.Type, req.Title, req.Content, req.Concepts, req.Files, req.Strength)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"memory": mem,
	})
}

// HandleForget handles POST /imprint/forget.
func (h *MemoryHandler) HandleForget(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.Forget(req.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// HandlePin handles POST /imprint/memories/pin. Body: {id, pinned}.
// Pinned memórias são imunes ao decay sweep — usado pra preservar
// conhecimento crítico que raramente é reforçado.
func (h *MemoryHandler) HandlePin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     string `json:"id"`
		Pinned bool   `json:"pinned"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	if err := h.svc.SetPinned(req.ID, req.Pinned); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": req.ID, "pinned": req.Pinned})
}

// HandleSetConcepts handles POST /imprint/memories/concepts. Body: {id, concepts: []string}.
// Substitui inteiramente os concepts da memória. Editor inline de tags da UI.
func (h *MemoryHandler) HandleSetConcepts(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID       string   `json:"id"`
		Concepts []string `json:"concepts"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	mem, err := h.svc.SetConcepts(req.ID, req.Concepts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"memory": mem})
}

// HandleClusterSummary handles POST /imprint/memories/cluster-summary.
// Body: {ids: [memId,...]}. Devolve um título curto (~5-8 palavras) que
// resume o tema comum às memórias do cluster. Resultado em cache pra
// evitar custo Haiku em hovers repetidos.
func (h *MemoryHandler) HandleClusterSummary(w http.ResponseWriter, r *http.Request) {
	if h.llm == nil {
		writeError(w, http.StatusServiceUnavailable, "no LLM provider configured")
		return
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.IDs) < 2 {
		writeError(w, http.StatusBadRequest, "need at least 2 ids")
		return
	}
	if len(req.IDs) > 30 {
		req.IDs = req.IDs[:30] // cap pra controlar custo
	}

	// Cache key: ids ordenados + concatenados. Hover na mesma community
	// 10x não dispara 10 calls Haiku.
	sorted := append([]string(nil), req.IDs...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j-1] > sorted[j]; j-- {
			sorted[j-1], sorted[j] = sorted[j], sorted[j-1]
		}
	}
	cacheKey := strings.Join(sorted, ",")
	h.clusterCacheMu.RLock()
	if cached, ok := h.clusterCache[cacheKey]; ok {
		h.clusterCacheMu.RUnlock()
		writeJSON(w, http.StatusOK, map[string]any{"summary": cached, "cached": true})
		return
	}
	h.clusterCacheMu.RUnlock()

	// Coleta títulos das memórias. Narrative é caro; só title é o
	// suficiente pra LLM descobrir o tema do cluster.
	titles := make([]string, 0, len(req.IDs))
	for _, id := range req.IDs {
		mem, err := h.svc.GetByID(id)
		if err != nil || mem == nil {
			continue
		}
		titles = append(titles, "- "+mem.Title)
	}
	if len(titles) < 2 {
		writeError(w, http.StatusBadRequest, "could not load enough memories")
		return
	}

	prompt := "Resuma o tema comum a estas memórias em UMA frase curta " +
		"de 4-7 palavras. Sem aspas, sem markdown, só a frase em PT-BR.\n\n" +
		strings.Join(titles, "\n")

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	resp, err := h.llm.Complete(ctx, llm.CompletionRequest{
		SystemPrompt: "Você é um sintetizador de cluster topic. Responda só com o título resumido, sem prefixos.",
		UserPrompt:   prompt,
		MaxTokens:    40,
		Temperature:  0.3,
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("LLM failed: %v", err))
		return
	}
	summary := strings.TrimSpace(resp)
	// Defesa básica: pode vir com aspas ou bullet — limpa.
	summary = strings.Trim(summary, "\"' \n\t-•")
	if summary == "" {
		writeError(w, http.StatusBadGateway, "LLM returned empty")
		return
	}

	h.clusterCacheMu.Lock()
	h.clusterCache[cacheKey] = summary
	h.clusterCacheMu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{"summary": summary, "cached": false})
}

// HandleEvolve handles POST /imprint/evolve. Body fields content / title /
// type / strength are all optional; missing ones inherit from the previous
// version.
func (h *MemoryHandler) HandleEvolve(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID       string `json:"id"`
		Content  string `json:"content"`
		Title    string `json:"title"`
		Type     string `json:"type"`
		Strength int    `json:"strength"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	mem, err := h.svc.Evolve(req.ID, service.EvolveInput{
		Content:  req.Content,
		Title:    req.Title,
		Type:     req.Type,
		Strength: req.Strength,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"memory": mem,
	})
}

// HandleList handles GET /imprint/memories.
// Aceita ?before=<RFC3339> para a feature time-travel da UI: filtra
// memórias com created_at <= before. Sem o parâmetro, comportamento atual.
func (h *MemoryHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	memType := r.URL.Query().Get("type")

	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	var before time.Time
	if v := r.URL.Query().Get("before"); v != "" {
		// Aceita RFC3339 completo (com offset) ou só YYYY-MM-DD por
		// conveniência. Inválido = ignora silenciosamente.
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			before = t
		} else if t, err := time.Parse("2006-01-02", v); err == nil {
			before = t
		}
	}

	memories, err := h.svc.List(memType, limit, offset, before)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	total, _ := h.svc.Count()

	writeJSON(w, http.StatusOK, map[string]any{
		"memories": memories,
		"total":    total,
	})
}

// HandleGraph handles GET /imprint/memories/graph?topN=200&minShared=1.
// Returns memory-centric graph nodes + edges for the Graph tab's "memories"
// view.
func (h *MemoryHandler) HandleGraph(w http.ResponseWriter, r *http.Request) {
	topN := 200
	minShared := 1
	if v := r.URL.Query().Get("topN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			topN = n
		}
	}
	if v := r.URL.Query().Get("minShared"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			minShared = n
		}
	}
	nodes, edges, err := h.svc.MemoryGraph(topN, minShared)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"nodes": nodes,
		"edges": edges,
	})
}

// HandleHistory handles GET /imprint/memories/history?id=mem_xxx. Returns
// every version of the memory in oldest-first order so the UI can render a
// timeline.
func (h *MemoryHandler) HandleHistory(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id query parameter is required")
		return
	}
	versions, err := h.svc.History(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": versions})
}

// HandleConcepts handles GET /imprint/memories/concepts. Returns the top
// concepts aggregated server-side from all latest memories.
func (h *MemoryHandler) HandleConcepts(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	concepts, err := h.svc.TopConcepts(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"concepts": concepts})
}
