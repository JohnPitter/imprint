package handler

import (
	"encoding/json"
	"net/http"

	"imprint/internal/config"
)

// SettingsHandler handles the settings API endpoints.
type SettingsHandler struct {
	cfg *config.Config
}

// NewSettingsHandler creates a new SettingsHandler.
func NewSettingsHandler(cfg *config.Config) *SettingsHandler {
	return &SettingsHandler{cfg: cfg}
}

// HandleGetSettings returns the current configuration (API keys masked).
func (h *SettingsHandler) HandleGetSettings(w http.ResponseWriter, r *http.Request) {
	view := config.ConfigToPublicView(h.cfg)
	writeJSON(w, http.StatusOK, view)
}

// HandleUpdateSettings saves user settings and returns the updated config.
func (h *SettingsHandler) HandleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var settings config.UserSettings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Save to disk
	if err := config.SaveUserSettings(h.cfg.DataDir, &settings); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save settings: "+err.Error())
		return
	}

	// Apply to running config
	config.ApplyUserSettings(h.cfg, &settings)

	// Return updated view
	view := config.ConfigToPublicView(h.cfg)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "saved",
		"settings": view,
		"note":     "LLM provider changes take effect on next observation. Restart the server for full effect.",
	})
}
