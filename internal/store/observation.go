package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// RawObservationRow represents the raw_observations table row.
type RawObservationRow struct {
	ID         string
	SessionID  string
	Timestamp  time.Time
	HookType   string
	ToolName   *string
	ToolInput  json.RawMessage
	ToolOutput json.RawMessage
	UserPrompt *string
	Raw        json.RawMessage
}

// CompressedObservationRow represents the compressed_observations table row.
type CompressedObservationRow struct {
	ID                  string
	SessionID           string
	Timestamp           time.Time
	Type                string
	Title               string
	Subtitle            *string
	Facts               []string
	Narrative           *string
	Concepts            []string
	Files               []string
	Importance          int
	Confidence          float64
	SourceObservationID *string
}

// ObservationStore handles raw and compressed observations.
type ObservationStore struct {
	db *DB
}

// NewObservationStore creates a new ObservationStore backed by the given DB.
func NewObservationStore(db *DB) *ObservationStore {
	return &ObservationStore{db: db}
}

// CreateRaw inserts a raw observation.
func (s *ObservationStore) CreateRaw(obs *RawObservationRow) error {
	_, err := s.db.Exec(
		`INSERT INTO raw_observations (id, session_id, timestamp, hook_type, tool_name, tool_input, tool_output, user_prompt, raw)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		obs.ID,
		obs.SessionID,
		TimeToString(obs.Timestamp),
		obs.HookType,
		NullString(obs.ToolName),
		marshalRawJSON(obs.ToolInput),
		marshalRawJSON(obs.ToolOutput),
		NullString(obs.UserPrompt),
		marshalRawJSON(obs.Raw),
	)
	if err != nil {
		return fmt.Errorf("raw observation create: %w", err)
	}
	return nil
}

// CreateCompressed inserts a compressed observation.
func (s *ObservationStore) CreateCompressed(obs *CompressedObservationRow) error {
	_, err := s.db.Exec(
		`INSERT INTO compressed_observations
		 (id, session_id, timestamp, type, title, subtitle, facts, narrative, concepts, files, importance, confidence, source_observation_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		obs.ID,
		obs.SessionID,
		TimeToString(obs.Timestamp),
		obs.Type,
		obs.Title,
		NullString(obs.Subtitle),
		MarshalJSON(obs.Facts),
		NullString(obs.Narrative),
		MarshalJSON(obs.Concepts),
		MarshalJSON(obs.Files),
		obs.Importance,
		obs.Confidence,
		NullString(obs.SourceObservationID),
	)
	if err != nil {
		return fmt.Errorf("compressed observation create: %w", err)
	}
	return nil
}

// ListRaw returns raw observations for a session, ordered by timestamp.
func (s *ObservationStore) ListRaw(sessionID string, limit, offset int) ([]RawObservationRow, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, timestamp, hook_type, tool_name, tool_input, tool_output, user_prompt, raw
		 FROM raw_observations WHERE session_id = ?
		 ORDER BY timestamp ASC LIMIT ? OFFSET ?`,
		sessionID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("raw observation list: %w", err)
	}
	defer rows.Close()

	return scanRawObservations(rows)
}

// ListCompressed returns compressed observations for a session, ordered by timestamp.
func (s *ObservationStore) ListCompressed(sessionID string, limit, offset int) ([]CompressedObservationRow, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, timestamp, type, title, subtitle, facts, narrative, concepts, files, importance, confidence, source_observation_id
		 FROM compressed_observations WHERE session_id = ?
		 ORDER BY timestamp ASC LIMIT ? OFFSET ?`,
		sessionID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("compressed observation list: %w", err)
	}
	defer rows.Close()

	return scanCompressedObservations(rows)
}

// CountBySession returns the raw observation count for a session.
func (s *ObservationStore) CountBySession(sessionID string) (int, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM raw_observations WHERE session_id = ?`, sessionID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("observation count by session: %w", err)
	}
	return count, nil
}

// GetCompressedByID returns a single compressed observation by ID.
func (s *ObservationStore) GetCompressedByID(id string) (*CompressedObservationRow, error) {
	row := s.db.QueryRow(
		`SELECT id, session_id, timestamp, type, title, subtitle, facts, narrative, concepts, files, importance, confidence, source_observation_id
		 FROM compressed_observations WHERE id = ?`, id,
	)
	return scanCompressedObservation(row)
}

// ListCompressedByImportance returns compressed observations with importance >= minImportance
// across sessions for a given project, ordered by timestamp DESC.
func (s *ObservationStore) ListCompressedByImportance(project string, minImportance int, limit int) ([]CompressedObservationRow, error) {
	rows, err := s.db.Query(
		`SELECT co.id, co.session_id, co.timestamp, co.type, co.title, co.subtitle,
		        co.facts, co.narrative, co.concepts, co.files, co.importance, co.confidence, co.source_observation_id
		 FROM compressed_observations co
		 JOIN sessions s ON co.session_id = s.id
		 WHERE s.project = ? AND co.importance >= ?
		 ORDER BY co.timestamp DESC
		 LIMIT ?`,
		project, minImportance, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("compressed observation list by importance: %w", err)
	}
	defer rows.Close()

	return scanCompressedObservations(rows)
}

// DeleteRawOlderThan removes raw observations older than the given time.
// Returns the number of deleted rows.
func (s *ObservationStore) DeleteRawOlderThan(before time.Time) (int64, error) {
	res, err := s.db.Exec(
		`DELETE FROM raw_observations WHERE timestamp < ?`,
		TimeToString(before),
	)
	if err != nil {
		return 0, fmt.Errorf("raw observation delete older than: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// InsertDedup attempts to insert a hash into dedup_cache.
// Returns true if the hash was inserted (new entry), false if it already existed (duplicate).
func (s *ObservationStore) InsertDedup(hash string) (bool, error) {
	res, err := s.db.Exec(
		`INSERT OR IGNORE INTO dedup_cache (hash, created_at) VALUES (?, ?)`,
		hash, TimeToString(time.Now()),
	)
	if err != nil {
		return false, fmt.Errorf("observation insert dedup: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// marshalRawJSON converts json.RawMessage to a sql.NullString for TEXT columns.
// Returns NULL when the raw message is nil or empty.
func marshalRawJSON(raw json.RawMessage) sql.NullString {
	if len(raw) == 0 {
		return sql.NullString{}
	}
	return sql.NullString{String: string(raw), Valid: true}
}

// scanRawObservation scans a single row into a RawObservationRow.
func scanRawObservations(rows *sql.Rows) ([]RawObservationRow, error) {
	var result []RawObservationRow

	for rows.Next() {
		var (
			obs        RawObservationRow
			ts         string
			toolName   sql.NullString
			toolInput  sql.NullString
			toolOutput sql.NullString
			userPrompt sql.NullString
			raw        sql.NullString
		)

		err := rows.Scan(
			&obs.ID,
			&obs.SessionID,
			&ts,
			&obs.HookType,
			&toolName,
			&toolInput,
			&toolOutput,
			&userPrompt,
			&raw,
		)
		if err != nil {
			return nil, fmt.Errorf("raw observation scan row: %w", err)
		}

		obs.Timestamp = ParseTime(ts)
		if toolName.Valid {
			obs.ToolName = &toolName.String
		}
		if toolInput.Valid {
			obs.ToolInput = json.RawMessage(toolInput.String)
		}
		if toolOutput.Valid {
			obs.ToolOutput = json.RawMessage(toolOutput.String)
		}
		if userPrompt.Valid {
			obs.UserPrompt = &userPrompt.String
		}
		if raw.Valid {
			obs.Raw = json.RawMessage(raw.String)
		}

		result = append(result, obs)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("raw observation scan rows: %w", err)
	}

	return result, nil
}

// scanCompressedObservation scans a single sql.Row into a CompressedObservationRow.
func scanCompressedObservation(row *sql.Row) (*CompressedObservationRow, error) {
	var (
		obs       CompressedObservationRow
		ts        string
		subtitle  sql.NullString
		facts     sql.NullString
		narrative sql.NullString
		concepts  sql.NullString
		files     sql.NullString
		sourceID  sql.NullString
	)

	err := row.Scan(
		&obs.ID,
		&obs.SessionID,
		&ts,
		&obs.Type,
		&obs.Title,
		&subtitle,
		&facts,
		&narrative,
		&concepts,
		&files,
		&obs.Importance,
		&obs.Confidence,
		&sourceID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("compressed observation not found")
		}
		return nil, fmt.Errorf("compressed observation scan: %w", err)
	}

	obs.Timestamp = ParseTime(ts)
	if subtitle.Valid {
		obs.Subtitle = &subtitle.String
	}
	if facts.Valid {
		obs.Facts = UnmarshalStringSlice(facts.String)
	}
	if narrative.Valid {
		obs.Narrative = &narrative.String
	}
	if concepts.Valid {
		obs.Concepts = UnmarshalStringSlice(concepts.String)
	}
	if files.Valid {
		obs.Files = UnmarshalStringSlice(files.String)
	}
	if sourceID.Valid {
		obs.SourceObservationID = &sourceID.String
	}

	return &obs, nil
}

// scanCompressedObservations scans multiple rows into a slice of CompressedObservationRow.
func scanCompressedObservations(rows *sql.Rows) ([]CompressedObservationRow, error) {
	var result []CompressedObservationRow

	for rows.Next() {
		var (
			obs       CompressedObservationRow
			ts        string
			subtitle  sql.NullString
			facts     sql.NullString
			narrative sql.NullString
			concepts  sql.NullString
			files     sql.NullString
			sourceID  sql.NullString
		)

		err := rows.Scan(
			&obs.ID,
			&obs.SessionID,
			&ts,
			&obs.Type,
			&obs.Title,
			&subtitle,
			&facts,
			&narrative,
			&concepts,
			&files,
			&obs.Importance,
			&obs.Confidence,
			&sourceID,
		)
		if err != nil {
			return nil, fmt.Errorf("compressed observation scan row: %w", err)
		}

		obs.Timestamp = ParseTime(ts)
		if subtitle.Valid {
			obs.Subtitle = &subtitle.String
		}
		if facts.Valid {
			obs.Facts = UnmarshalStringSlice(facts.String)
		}
		if narrative.Valid {
			obs.Narrative = &narrative.String
		}
		if concepts.Valid {
			obs.Concepts = UnmarshalStringSlice(concepts.String)
		}
		if files.Valid {
			obs.Files = UnmarshalStringSlice(files.String)
		}
		if sourceID.Valid {
			obs.SourceObservationID = &sourceID.String
		}

		result = append(result, obs)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("compressed observation scan rows: %w", err)
	}

	return result, nil
}
