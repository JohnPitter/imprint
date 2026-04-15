package types

import (
	"encoding/json"
	"time"
)

// Session represents an active or completed agent session.
type Session struct {
	ID          string        `json:"id" db:"id"`
	ProjectDir  string        `json:"project_dir" db:"project_dir"`
	Status      SessionStatus `json:"status" db:"status"`
	ParentID    *string       `json:"parent_id,omitempty" db:"parent_id"`
	Metadata    json.RawMessage `json:"metadata,omitempty" db:"metadata"`
	StartedAt   time.Time     `json:"started_at" db:"started_at"`
	EndedAt     *time.Time    `json:"ended_at,omitempty" db:"ended_at"`
}

// RawObservation is an unprocessed observation captured from a hook event.
type RawObservation struct {
	ID        string          `json:"id" db:"id"`
	SessionID string          `json:"session_id" db:"session_id"`
	Type      ObservationType `json:"type" db:"type"`
	Content   string          `json:"content" db:"content"`
	ToolName  *string         `json:"tool_name,omitempty" db:"tool_name"`
	FilePath  *string         `json:"file_path,omitempty" db:"file_path"`
	Metadata  json.RawMessage `json:"metadata,omitempty" db:"metadata"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
}

// CompressedObservation is a summarized batch of raw observations.
type CompressedObservation struct {
	ID              string          `json:"id" db:"id"`
	SessionID       string          `json:"session_id" db:"session_id"`
	Summary         string          `json:"summary" db:"summary"`
	KeyActions      json.RawMessage `json:"key_actions" db:"key_actions"`
	FilesTouched    json.RawMessage `json:"files_touched" db:"files_touched"`
	RawIDs          json.RawMessage `json:"raw_ids" db:"raw_ids"`
	ObservationCount int            `json:"observation_count" db:"observation_count"`
	Metadata        json.RawMessage `json:"metadata,omitempty" db:"metadata"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
}

// Memory is a distilled long-term memory extracted from observations.
type Memory struct {
	ID         string          `json:"id" db:"id"`
	Type       MemoryType      `json:"type" db:"type"`
	Content    string          `json:"content" db:"content"`
	Tags       json.RawMessage `json:"tags" db:"tags"`
	Confidence float64         `json:"confidence" db:"confidence"`
	AccessCount int            `json:"access_count" db:"access_count"`
	Decay      float64         `json:"decay" db:"decay"`
	SourceIDs  json.RawMessage `json:"source_ids" db:"source_ids"`
	ProjectDir *string         `json:"project_dir,omitempty" db:"project_dir"`
	Metadata   json.RawMessage `json:"metadata,omitempty" db:"metadata"`
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at" db:"updated_at"`
	LastAccess time.Time       `json:"last_access" db:"last_access"`
}

// SemanticMemory holds a memory's vector embedding for similarity search.
type SemanticMemory struct {
	ID        string    `json:"id" db:"id"`
	MemoryID  string    `json:"memory_id" db:"memory_id"`
	Embedding []float64 `json:"embedding" db:"embedding"`
	Model     string    `json:"model" db:"model"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// ProceduralMemory stores step-by-step procedures learned from sessions.
type ProceduralMemory struct {
	ID          string          `json:"id" db:"id"`
	Name        string          `json:"name" db:"name"`
	Description string          `json:"description" db:"description"`
	Steps       json.RawMessage `json:"steps" db:"steps"`
	TriggerPattern string       `json:"trigger_pattern" db:"trigger_pattern"`
	SuccessCount int            `json:"success_count" db:"success_count"`
	FailureCount int            `json:"failure_count" db:"failure_count"`
	ProjectDir  *string         `json:"project_dir,omitempty" db:"project_dir"`
	Metadata    json.RawMessage `json:"metadata,omitempty" db:"metadata"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
}

// SessionSummary is a condensed summary generated at session end.
type SessionSummary struct {
	ID            string          `json:"id" db:"id"`
	SessionID     string          `json:"session_id" db:"session_id"`
	Summary       string          `json:"summary" db:"summary"`
	KeyDecisions  json.RawMessage `json:"key_decisions" db:"key_decisions"`
	FilesTouched  json.RawMessage `json:"files_touched" db:"files_touched"`
	ErrorsHit     json.RawMessage `json:"errors_hit" db:"errors_hit"`
	MemoriesFormed json.RawMessage `json:"memories_formed" db:"memories_formed"`
	Duration      int             `json:"duration" db:"duration"`
	Metadata      json.RawMessage `json:"metadata,omitempty" db:"metadata"`
	CreatedAt     time.Time       `json:"created_at" db:"created_at"`
}

// GraphNode is a vertex in the knowledge graph.
type GraphNode struct {
	ID         string          `json:"id" db:"id"`
	Name       string          `json:"name" db:"name"`
	Type       GraphNodeType   `json:"type" db:"type"`
	Properties json.RawMessage `json:"properties,omitempty" db:"properties"`
	Weight     float64         `json:"weight" db:"weight"`
	ProjectDir *string         `json:"project_dir,omitempty" db:"project_dir"`
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at" db:"updated_at"`
}

// GraphEdge is a directed relationship between two graph nodes.
type GraphEdge struct {
	ID         string          `json:"id" db:"id"`
	SourceID   string          `json:"source_id" db:"source_id"`
	TargetID   string          `json:"target_id" db:"target_id"`
	Type       GraphEdgeType   `json:"type" db:"type"`
	Weight     float64         `json:"weight" db:"weight"`
	Properties json.RawMessage `json:"properties,omitempty" db:"properties"`
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
}

// Action is a tracked task or to-do item surfaced from agent activity.
type Action struct {
	ID          string          `json:"id" db:"id"`
	SessionID   *string         `json:"session_id,omitempty" db:"session_id"`
	Title       string          `json:"title" db:"title"`
	Description string          `json:"description" db:"description"`
	Status      ActionStatus    `json:"status" db:"status"`
	Priority    int             `json:"priority" db:"priority"`
	ProjectDir  *string         `json:"project_dir,omitempty" db:"project_dir"`
	Tags        json.RawMessage `json:"tags" db:"tags"`
	DueAt       *time.Time      `json:"due_at,omitempty" db:"due_at"`
	Metadata    json.RawMessage `json:"metadata,omitempty" db:"metadata"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
	CompletedAt *time.Time      `json:"completed_at,omitempty" db:"completed_at"`
}

// ActionEdge represents a dependency between two actions.
type ActionEdge struct {
	ID       string `json:"id" db:"id"`
	SourceID string `json:"source_id" db:"source_id"`
	TargetID string `json:"target_id" db:"target_id"`
	Type     string `json:"type" db:"type"`
}

// Lease provides distributed locking for multi-agent coordination.
type Lease struct {
	ID         string      `json:"id" db:"id"`
	ResourceID string      `json:"resource_id" db:"resource_id"`
	HolderID   string      `json:"holder_id" db:"holder_id"`
	Status     LeaseStatus `json:"status" db:"status"`
	ExpiresAt  time.Time   `json:"expires_at" db:"expires_at"`
	AcquiredAt time.Time   `json:"acquired_at" db:"acquired_at"`
	ReleasedAt *time.Time  `json:"released_at,omitempty" db:"released_at"`
}

// Routine is a scheduled or repeatable sequence of steps.
type Routine struct {
	ID          string          `json:"id" db:"id"`
	Name        string          `json:"name" db:"name"`
	Description string          `json:"description" db:"description"`
	Steps       json.RawMessage `json:"steps" db:"steps"`
	Schedule    *string         `json:"schedule,omitempty" db:"schedule"`
	Enabled     bool            `json:"enabled" db:"enabled"`
	LastRunAt   *time.Time      `json:"last_run_at,omitempty" db:"last_run_at"`
	ProjectDir  *string         `json:"project_dir,omitempty" db:"project_dir"`
	Metadata    json.RawMessage `json:"metadata,omitempty" db:"metadata"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
}

// RoutineStep is a single step within a routine execution.
type RoutineStep struct {
	ID        string          `json:"id" db:"id"`
	RoutineID string          `json:"routine_id" db:"routine_id"`
	StepIndex int             `json:"step_index" db:"step_index"`
	Action    string          `json:"action" db:"action"`
	Input     json.RawMessage `json:"input,omitempty" db:"input"`
	Output    json.RawMessage `json:"output,omitempty" db:"output"`
	Status    string          `json:"status" db:"status"`
	Error     *string         `json:"error,omitempty" db:"error"`
	StartedAt time.Time       `json:"started_at" db:"started_at"`
	EndedAt   *time.Time      `json:"ended_at,omitempty" db:"ended_at"`
}

// Signal is an inter-agent or inter-session message.
type Signal struct {
	ID        string          `json:"id" db:"id"`
	Type      string          `json:"type" db:"type"`
	Source    string          `json:"source" db:"source"`
	Target    *string         `json:"target,omitempty" db:"target"`
	Payload   json.RawMessage `json:"payload" db:"payload"`
	Consumed  bool            `json:"consumed" db:"consumed"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
	ExpiresAt *time.Time      `json:"expires_at,omitempty" db:"expires_at"`
}

// Checkpoint is a human-gated approval point in a workflow.
type Checkpoint struct {
	ID          string           `json:"id" db:"id"`
	SessionID   string           `json:"session_id" db:"session_id"`
	Title       string           `json:"title" db:"title"`
	Description string           `json:"description" db:"description"`
	Status      CheckpointStatus `json:"status" db:"status"`
	Payload     json.RawMessage  `json:"payload,omitempty" db:"payload"`
	Response    json.RawMessage  `json:"response,omitempty" db:"response"`
	CreatedAt   time.Time        `json:"created_at" db:"created_at"`
	ResolvedAt  *time.Time       `json:"resolved_at,omitempty" db:"resolved_at"`
}

// Sentinel watches for conditions and triggers actions when met.
type Sentinel struct {
	ID         string          `json:"id" db:"id"`
	Name       string          `json:"name" db:"name"`
	Type       SentinelType    `json:"type" db:"type"`
	Status     SentinelStatus  `json:"status" db:"status"`
	Condition  json.RawMessage `json:"condition" db:"condition"`
	Action     json.RawMessage `json:"action" db:"action"`
	ProjectDir *string         `json:"project_dir,omitempty" db:"project_dir"`
	LastCheck  *time.Time      `json:"last_check,omitempty" db:"last_check"`
	TriggeredAt *time.Time     `json:"triggered_at,omitempty" db:"triggered_at"`
	ExpiresAt  *time.Time      `json:"expires_at,omitempty" db:"expires_at"`
	Metadata   json.RawMessage `json:"metadata,omitempty" db:"metadata"`
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at" db:"updated_at"`
}

// Sketch is a temporary working hypothesis or draft that can be promoted to a memory.
type Sketch struct {
	ID         string          `json:"id" db:"id"`
	SessionID  string          `json:"session_id" db:"session_id"`
	Content    string          `json:"content" db:"content"`
	Status     SketchStatus    `json:"status" db:"status"`
	Confidence float64         `json:"confidence" db:"confidence"`
	PromotedTo *string         `json:"promoted_to,omitempty" db:"promoted_to"`
	Metadata   json.RawMessage `json:"metadata,omitempty" db:"metadata"`
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at" db:"updated_at"`
	ExpiresAt  *time.Time      `json:"expires_at,omitempty" db:"expires_at"`
}

// Crystal is a highly refined, stable insight distilled from many memories.
type Crystal struct {
	ID          string          `json:"id" db:"id"`
	Title       string          `json:"title" db:"title"`
	Content     string          `json:"content" db:"content"`
	SourceIDs   json.RawMessage `json:"source_ids" db:"source_ids"`
	Confidence  float64         `json:"confidence" db:"confidence"`
	AccessCount int             `json:"access_count" db:"access_count"`
	ProjectDir  *string         `json:"project_dir,omitempty" db:"project_dir"`
	Metadata    json.RawMessage `json:"metadata,omitempty" db:"metadata"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
}

// Lesson captures a learned outcome from a success or failure.
type Lesson struct {
	ID         string          `json:"id" db:"id"`
	SessionID  *string         `json:"session_id,omitempty" db:"session_id"`
	Title      string          `json:"title" db:"title"`
	Content    string          `json:"content" db:"content"`
	Category   string          `json:"category" db:"category"`
	Outcome    string          `json:"outcome" db:"outcome"`
	Tags       json.RawMessage `json:"tags" db:"tags"`
	ProjectDir *string         `json:"project_dir,omitempty" db:"project_dir"`
	Metadata   json.RawMessage `json:"metadata,omitempty" db:"metadata"`
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
}

// Insight is an automatically generated analytical observation.
type Insight struct {
	ID         string          `json:"id" db:"id"`
	Title      string          `json:"title" db:"title"`
	Content    string          `json:"content" db:"content"`
	Category   string          `json:"category" db:"category"`
	Severity   string          `json:"severity" db:"severity"`
	SourceIDs  json.RawMessage `json:"source_ids" db:"source_ids"`
	ProjectDir *string         `json:"project_dir,omitempty" db:"project_dir"`
	Metadata   json.RawMessage `json:"metadata,omitempty" db:"metadata"`
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
}

// Facet represents a personality or behavioral dimension of the agent.
type Facet struct {
	ID          string          `json:"id" db:"id"`
	Name        string          `json:"name" db:"name"`
	Description string          `json:"description" db:"description"`
	Value       float64         `json:"value" db:"value"`
	Evidence    json.RawMessage `json:"evidence" db:"evidence"`
	ProjectDir  *string         `json:"project_dir,omitempty" db:"project_dir"`
	Metadata    json.RawMessage `json:"metadata,omitempty" db:"metadata"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
}

// AuditEntry logs every significant operation for traceability.
type AuditEntry struct {
	ID         string          `json:"id" db:"id"`
	SessionID  *string         `json:"session_id,omitempty" db:"session_id"`
	Action     AuditAction     `json:"action" db:"action"`
	EntityType string          `json:"entity_type" db:"entity_type"`
	EntityID   *string         `json:"entity_id,omitempty" db:"entity_id"`
	Details    json.RawMessage `json:"details,omitempty" db:"details"`
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
}

// ProjectProfile stores per-project configuration and preferences.
type ProjectProfile struct {
	ID          string          `json:"id" db:"id"`
	ProjectDir  string          `json:"project_dir" db:"project_dir"`
	Name        string          `json:"name" db:"name"`
	Description string          `json:"description" db:"description"`
	Language    *string         `json:"language,omitempty" db:"language"`
	Framework   *string         `json:"framework,omitempty" db:"framework"`
	Conventions json.RawMessage `json:"conventions,omitempty" db:"conventions"`
	Metadata    json.RawMessage `json:"metadata,omitempty" db:"metadata"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
}

// MeshPeer represents a remote imprint instance for data synchronization.
type MeshPeer struct {
	ID         string          `json:"id" db:"id"`
	Name       string          `json:"name" db:"name"`
	Endpoint   string          `json:"endpoint" db:"endpoint"`
	APIKey     string          `json:"api_key" db:"api_key"`
	Status     string          `json:"status" db:"status"`
	LastSyncAt *time.Time      `json:"last_sync_at,omitempty" db:"last_sync_at"`
	Metadata   json.RawMessage `json:"metadata,omitempty" db:"metadata"`
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at" db:"updated_at"`
}

// ---------------------------------------------------------------------------
// Composite / transport types (not directly stored as single tables)
// ---------------------------------------------------------------------------

// ExportData is a versioned container for full data export and import.
type ExportData struct {
	Version               string                  `json:"version"`
	ExportedAt            time.Time               `json:"exported_at"`
	Sessions              []Session               `json:"sessions"`
	RawObservations       []RawObservation        `json:"raw_observations"`
	CompressedObservations []CompressedObservation `json:"compressed_observations"`
	Memories              []Memory                `json:"memories"`
	SemanticMemories      []SemanticMemory        `json:"semantic_memories"`
	ProceduralMemories    []ProceduralMemory      `json:"procedural_memories"`
	SessionSummaries      []SessionSummary        `json:"session_summaries"`
	GraphNodes            []GraphNode             `json:"graph_nodes"`
	GraphEdges            []GraphEdge             `json:"graph_edges"`
	Actions               []Action                `json:"actions"`
	ActionEdges           []ActionEdge            `json:"action_edges"`
	Leases                []Lease                 `json:"leases"`
	Routines              []Routine               `json:"routines"`
	Signals               []Signal                `json:"signals"`
	Checkpoints           []Checkpoint            `json:"checkpoints"`
	Sentinels             []Sentinel              `json:"sentinels"`
	Sketches              []Sketch                `json:"sketches"`
	Crystals              []Crystal               `json:"crystals"`
	Lessons               []Lesson                `json:"lessons"`
	Insights              []Insight               `json:"insights"`
	Facets                []Facet                 `json:"facets"`
	AuditLog              []AuditEntry            `json:"audit_log"`
	ProjectProfiles       []ProjectProfile        `json:"project_profiles"`
	MeshPeers             []MeshPeer              `json:"mesh_peers"`
}

// DashboardStats provides aggregated statistics for the UI dashboard.
type DashboardStats struct {
	TotalSessions     int            `json:"total_sessions"`
	ActiveSessions    int            `json:"active_sessions"`
	TotalMemories     int            `json:"total_memories"`
	TotalObservations int            `json:"total_observations"`
	TotalGraphNodes   int            `json:"total_graph_nodes"`
	TotalGraphEdges   int            `json:"total_graph_edges"`
	TotalActions      int            `json:"total_actions"`
	PendingActions    int            `json:"pending_actions"`
	MemoryByType      map[string]int `json:"memory_by_type"`
	RecentSessions    []Session      `json:"recent_sessions"`
	TopMemories       []Memory       `json:"top_memories"`
}

// SearchResult wraps a memory with its relevance score.
type SearchResult struct {
	Memory Memory  `json:"memory"`
	Score  float64 `json:"score"`
	Source string  `json:"source"`
}

// HybridSearchResult combines keyword and semantic search results.
type HybridSearchResult struct {
	Results      []SearchResult `json:"results"`
	Query        string         `json:"query"`
	TotalResults int            `json:"total_results"`
	SearchTimeMs int64          `json:"search_time_ms"`
}

// ContextBlock represents a segment of context assembled for injection.
type ContextBlock struct {
	Type     string `json:"type"`
	Label    string `json:"label"`
	Content  string `json:"content"`
	Priority int    `json:"priority"`
}

// HookPayload is the JSON structure sent by hook binaries to the /observe endpoint.
type HookPayload struct {
	SessionID    string          `json:"session_id"`
	HookType     HookType        `json:"hook_type"`
	ProjectDir   string          `json:"project_dir"`
	ToolName     *string         `json:"tool_name,omitempty"`
	ToolInput    json.RawMessage `json:"tool_input,omitempty"`
	ToolOutput   json.RawMessage `json:"tool_output,omitempty"`
	UserPrompt   *string         `json:"user_prompt,omitempty"`
	FilePath     *string         `json:"file_path,omitempty"`
	Error        *string         `json:"error,omitempty"`
	Prompt       *string         `json:"prompt,omitempty"`
	Notification *string         `json:"notification,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
	Timestamp    time.Time       `json:"timestamp"`
}
