package service

import (
	"encoding/json"
	"time"

	"imprint/internal/store"

	"github.com/google/uuid"
)

// LogAudit writes an audit log entry. Failures are silently ignored.
func (c *Container) LogAudit(action, entityID, entityType string, meta map[string]any) {
	metaJSON, _ := json.Marshal(meta)
	c.Audit.Create(&store.AuditRow{
		ID:         "aud_" + uuid.New().String()[:8],
		Action:     action,
		EntityID:   entityID,
		EntityType: entityType,
		Meta:       json.RawMessage(metaJSON),
		Timestamp:  store.TimeToString(time.Now()),
	})
}
