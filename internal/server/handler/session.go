package handler

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"

	"imprint/internal/service"
	"imprint/internal/store"
	"imprint/internal/types"
)

// SessionHandler holds HTTP handlers for session endpoints.
// O Container é usado pelo HandleTimeline pra puxar observations,
// memories e actions vinculadas à sessão sem dependência circular.
type SessionHandler struct {
	svc       *service.SessionService
	container *service.Container
	tracker   *service.SessionTracker
}

// NewSessionHandler creates a new SessionHandler.
func NewSessionHandler(svc *service.SessionService, container *service.Container, tracker *service.SessionTracker) *SessionHandler {
	return &SessionHandler{svc: svc, container: container, tracker: tracker}
}

// HandleStart handles POST /imprint/session/start.
func (h *SessionHandler) HandleStart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"sessionId"`
		Project   string `json:"project"`
		Cwd       string `json:"cwd"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	session, context, err := h.svc.Start(req.SessionID, req.Project, req.Cwd)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if h.tracker != nil {
		h.tracker.Touch(session.ID)
	}

	contextStr := buildContextXML(context, req.Project)
	writeJSON(w, http.StatusOK, map[string]any{
		"session": session,
		"context": contextStr,
	})
}

// HandleHeartbeat handles POST /imprint/session/heartbeat.
// Chamado pelo Stop hook a cada turno do assistente; serve como sinal de vida
// pra que o scheduler saiba quando a sessão ficou idle e disparar finalize.
// Why: o hook SessionEnd do Claude Code não dispara consistentemente no /exit,
// então usamos ausência de heartbeat como proxy de "sessão terminou".
//
// Resurrection: se a sessão já foi finalizada (status='completed') mas o
// usuário voltou a interagir (ex: largou a janela aberta >5min jogando
// Valorant, voltou e fez pergunta nova), re-marcamos como active. O audit
// log retém o session.end original; session.reactivate é registrado.
func (h *SessionHandler) HandleHeartbeat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.SessionID == "" {
		writeError(w, http.StatusBadRequest, "sessionId is required")
		return
	}

	// Ressuscita se a sessão foi finalizada mas continua viva no Claude Code.
	// Falha de GetByID (sessão nunca criada) é benigna — o heartbeat ainda
	// alimenta o tracker e o sweep de idle vai limpá-la depois.
	if sess, err := h.svc.GetByID(req.SessionID); err == nil && sess.Status == types.SessionCompleted {
		if err := h.svc.Reactivate(req.SessionID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	if h.tracker != nil {
		h.tracker.Touch(req.SessionID)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// HandleEnd handles POST /imprint/session/end.
func (h *SessionHandler) HandleEnd(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.End(req.SessionID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if h.tracker != nil {
		h.tracker.Forget(req.SessionID)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// timelineEvent é um item homogêneo que mistura compressed_observations,
// memories e actions numa lista ordenada por tempo. O campo `kind` permite
// ao frontend renderizar com ícone/cor específicos.
type timelineEvent struct {
	Kind      string `json:"kind"`      // "observation" | "memory" | "action"
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"` // RFC3339; usado pra ordenação cronológica
	Title     string `json:"title"`
	Subtitle  string `json:"subtitle,omitempty"`
	Type      string `json:"type,omitempty"` // observation type / memory type / action status
	Score     int    `json:"score,omitempty"` // importance/strength/priority por kind
}

// HandleTimeline handles GET /imprint/sessions/timeline?id=ses_xxx.
// Retorna uma timeline cronológica unificada da sessão (observations
// comprimidas + memórias criadas com session_id no array + actions com
// session_id), ordenada do mais antigo pro mais recente. Útil pra
// "playback" de uma sessão concluída.
func (h *SessionHandler) HandleTimeline(w http.ResponseWriter, r *http.Request) {
	sid := r.URL.Query().Get("id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	limit := 500
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 && parsed <= 2000 {
			limit = parsed
		}
	}

	events := make([]timelineEvent, 0, limit)

	// 1) Compressed observations (sempre filtra por session_id direto).
	if h.container != nil {
		obs, err := h.container.Observations.ListCompressed(sid, limit, 0)
		if err == nil {
			for _, o := range obs {
				ev := timelineEvent{
					Kind:      "observation",
					ID:        o.ID,
					Timestamp: o.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
					Title:     o.Title,
					Type:      o.Type,
					Score:     o.Importance,
				}
				if o.Subtitle != nil {
					ev.Subtitle = *o.Subtitle
				}
				events = append(events, ev)
			}
		}

		// 2) Actions vinculadas à sessão (kanban).
		// Mais simples: pegamos todas as actions do projeto via
		// list filtrada por status arbitrário e checamos session_id.
		// O store hoje não tem ListBySession; faço o filtro client-side
		// porque o N de actions por sessão é tipicamente <50.
		statuses := []string{"pending", "in_progress", "done"}
		for _, st := range statuses {
			rows, err := h.container.Actions.List(st, "", 200, 0)
			if err != nil {
				continue
			}
			for _, a := range rows {
				if a.SessionID == nil || *a.SessionID != sid {
					continue
				}
				events = append(events, timelineEvent{
					Kind:      "action",
					ID:        a.ID,
					Timestamp: a.CreatedAt,
					Title:     a.Title,
					Subtitle:  a.Description,
					Type:      a.Status,
					Score:     a.Priority,
				})
			}
		}

		// 3) Memories que mencionam essa sessão em session_ids.
		// Usa LIKE no JSON-array — barato pro tamanho típico do DB.
		mems, err := h.container.Memories.ListBySessionID(sid, 200)
		if err == nil {
			for _, m := range mems {
				events = append(events, timelineEvent{
					Kind:      "memory",
					ID:        m.ID,
					Timestamp: m.CreatedAt,
					Title:     m.Title,
					Subtitle:  m.Content,
					Type:      m.Type,
					Score:     m.Strength,
				})
			}
		}
	}

	sort.Slice(events, func(i, j int) bool { return events[i].Timestamp < events[j].Timestamp })
	if len(events) > limit {
		events = events[:limit]
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"sessionId": sid,
		"events":    events,
		"count":     len(events),
	})
}

// _ assertion: garante store.MemoryRow.CreatedAt ainda é string.
var _ = store.MemoryRow{}.CreatedAt

// HandleList handles GET /imprint/sessions.
func (h *SessionHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")

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

	sessions, err := h.svc.List(project, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Total is independent of pagination — clients use it to render counts
	// without fetching every row.
	total, _ := h.svc.Count(project)

	writeJSON(w, http.StatusOK, map[string]any{
		"sessions": sessions,
		"total":    total,
	})
}
