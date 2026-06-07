package handler

import (
	"net/http"
	"strconv"

	"imprint/internal/llm"
	"imprint/internal/store"
)

// EconomyConfig is the plan/pricing context the handler needs to translate the
// raw token saldo into the number that matters per plan (currency for API,
// breathing room for Pro/Max).
type EconomyConfig struct {
	Plan            string  // "api" | "pro" | "max" (already resolved)
	PriceInPerMTok  float64 // USD per 1M input tokens
	PriceOutPerMTok float64 // USD per 1M output tokens
	ContextWindow   int     // nominal window size for the fôlego estimate
}

// EconomyHandler exposes the Phase 1 token economy meter: the saldo (context
// saved − Haiku spent) per repo/window, plus the plan-aware top number and the
// budget ceiling status.
type EconomyHandler struct {
	ledger *store.LedgerStore
	cfg    EconomyConfig
}

func NewEconomyHandler(ledger *store.LedgerStore, cfg EconomyConfig) *EconomyHandler {
	if cfg.ContextWindow <= 0 {
		cfg.ContextWindow = 200000 // Claude context window, used only for the fôlego %
	}
	return &EconomyHandler{ledger: ledger, cfg: cfg}
}

// economyResponse is the economy summary plus the plan-aware framing.
type economyResponse struct {
	store.EconomySummary
	Plan          string           `json:"plan"`
	SaldoMoedaUSD float64          `json:"saldoMoedaUSD"` // API plan: currency balance
	HaikuCostUSD  float64          `json:"haikuCostUSD"`  // what the Haiku spend cost
	FolegoPct     float64          `json:"folegoPct"`     // Pro/Max plan: window freed estimate
	Budget        llm.BudgetStatus `json:"budget"`
}

// HandleEconomy serves GET /imprint/economy?project=&sinceDays=.
// Empty project = all repos; sinceDays <= 0 = all time. The balance is reported
// honestly, including negative values during cold start (decision 3.4).
func (h *EconomyHandler) HandleEconomy(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	sinceDays := 0
	if v := r.URL.Query().Get("sinceDays"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			sinceDays = n
		}
	}

	summary, err := h.ledger.Economy(project, sinceDays)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// API plan: convert the saldo to currency. saved tokens are valued at the
	// (higher) output price as a conservative floor on avoided cost; the Haiku
	// spend uses its real input/output split.
	haikuCost := perMTok(summary.HaikuInput, h.cfg.PriceInPerMTok) + perMTok(summary.HaikuOutput, h.cfg.PriceOutPerMTok)
	savedValue := perMTok(summary.SavedTokens, h.cfg.PriceInPerMTok)
	saldoMoeda := savedValue - haikuCost

	// Pro/Max plan: fôlego = saved context as a fraction of the window.
	folego := 0.0
	if h.cfg.ContextWindow > 0 {
		folego = float64(summary.SavedTokens) / float64(h.cfg.ContextWindow) * 100
	}

	writeJSON(w, http.StatusOK, economyResponse{
		EconomySummary: summary,
		Plan:           h.cfg.Plan,
		SaldoMoedaUSD:  saldoMoeda,
		HaikuCostUSD:   haikuCost,
		FolegoPct:      folego,
		Budget:         llm.GlobalBudget.Status(),
	})
}

// perMTok returns the USD cost of n tokens at the given price-per-million.
func perMTok(n int64, pricePerMTok float64) float64 {
	return float64(n) / 1_000_000 * pricePerMTok
}
