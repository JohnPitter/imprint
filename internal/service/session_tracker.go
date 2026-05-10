package service

import (
	"sync"
	"time"
)

// SessionTracker registra o último heartbeat de cada sessão pra que o scheduler
// possa disparar finalize quando a sessão fica idle. Substitui a dependência
// no hook SessionEnd do Claude Code, que se mostrou não-confiável.
//
// Why: o hook /imprint/session/end + /imprint/finalize só dispara se o Claude
// Code emite o evento SessionEnd no /exit, o que não acontece consistentemente.
// Em vez disso, o hook Stop (que dispara a cada turno) faz heartbeat aqui, e
// o scheduler decide quando a sessão "morreu" pela ausência de heartbeats.
type SessionTracker struct {
	mu      sync.Mutex
	lastSeen map[string]time.Time
}

// NewSessionTracker cria um tracker vazio.
func NewSessionTracker() *SessionTracker {
	return &SessionTracker{lastSeen: make(map[string]time.Time)}
}

// Touch marca a sessão como ativa agora. Idempotente — chamado por todo POST
// /imprint/session/heartbeat.
func (t *SessionTracker) Touch(sessionID string) {
	if sessionID == "" {
		return
	}
	t.mu.Lock()
	t.lastSeen[sessionID] = time.Now()
	t.mu.Unlock()
}

// HasHeartbeat retorna true se a sessão já mandou pelo menos um heartbeat
// desde que o servidor subiu. Usado pelo scheduler pra distinguir sessões
// "vivas no tracker" de sessões DB-active órfãs (pré-restart).
func (t *SessionTracker) HasHeartbeat(sessionID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, ok := t.lastSeen[sessionID]
	return ok
}

// Forget remove a sessão do tracker. Chamado quando o scheduler já processou
// finalize, pra não tentar de novo até a próxima atividade.
func (t *SessionTracker) Forget(sessionID string) {
	t.mu.Lock()
	delete(t.lastSeen, sessionID)
	t.mu.Unlock()
}

// IdleSessions devolve as sessões cujo último heartbeat foi há mais de
// `threshold`. Snapshot — o caller pode iterar com segurança sem segurar o
// lock.
func (t *SessionTracker) IdleSessions(threshold time.Duration) []string {
	cutoff := time.Now().Add(-threshold)
	t.mu.Lock()
	defer t.mu.Unlock()

	idle := make([]string, 0, len(t.lastSeen))
	for sid, ts := range t.lastSeen {
		if ts.Before(cutoff) {
			idle = append(idle, sid)
		}
	}
	return idle
}
