package service

import (
	"encoding/json"
	"strings"
	"time"

	"imprint/internal/store"

	"github.com/google/uuid"
)

// LogAudit writes an audit log entry. Failures are silently ignored.
// If a WAL is attached, the entry is also appended to the file-based write-ahead log.
func (c *Container) LogAudit(action, entityID, entityType string, meta map[string]any) {
	metaJSON, _ := json.Marshal(meta)
	ts := store.TimeToString(time.Now())

	c.Audit.Create(&store.AuditRow{
		ID:         "aud_" + uuid.New().String()[:8],
		Action:     action,
		EntityID:   entityID,
		EntityType: entityType,
		Meta:       json.RawMessage(metaJSON),
		Timestamp:  ts,
	})

	// Mirror to file-based WAL for crash recovery and poisoning detection.
	if c.WAL != nil {
		contentLen := len(metaJSON)
		c.WAL.Log(store.WALEntry{
			Timestamp:  ts,
			Operation:  extractOperation(action),
			Entity:     entityType,
			EntityID:   entityID,
			ContentLen: contentLen,
			Meta:       meta,
		})
	}
}

// extractOperation extracts the operation verb from an audit action string.
// e.g. "observation.create" -> "create", "memory.delete" -> "delete".
func extractOperation(action string) string {
	parts := strings.SplitN(action, ".", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return action
}
