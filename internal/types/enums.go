package types

type HookType string

const (
	HookSessionStart    HookType = "session_start"
	HookPromptSubmit    HookType = "prompt_submit"
	HookPreToolUse      HookType = "pre_tool_use"
	HookPostToolUse     HookType = "post_tool_use"
	HookPostToolFailure HookType = "post_tool_failure"
	HookPreCompact      HookType = "pre_compact"
	HookSubagentStart   HookType = "subagent_start"
	HookSubagentStop    HookType = "subagent_stop"
	HookNotification    HookType = "notification"
	HookTaskCompleted   HookType = "task_completion"
	HookStop            HookType = "stop"
	HookSessionEnd      HookType = "session_end"
)

type ObservationType string

const (
	ObsFileOperation    ObservationType = "file_operation"
	ObsCommandExecution ObservationType = "command_execution"
	ObsSearch           ObservationType = "search"
	ObsConversation     ObservationType = "conversation"
	ObsError            ObservationType = "error"
	ObsDecision         ObservationType = "decision"
	ObsDiscovery        ObservationType = "discovery"
	ObsNotification     ObservationType = "notification"
	ObsTask             ObservationType = "task"
	ObsSubagentEvent    ObservationType = "subagent_event"
	ObsPermissionPrompt ObservationType = "permission_prompt"
	ObsTaskCompletion   ObservationType = "task_completion"
	ObsCompaction       ObservationType = "compaction"
	ObsOther            ObservationType = "other"
)

type MemoryType string

const (
	MemPattern      MemoryType = "pattern"
	MemPreference   MemoryType = "preference"
	MemArchitecture MemoryType = "architecture"
	MemBug          MemoryType = "bug"
	MemWorkflow     MemoryType = "workflow"
	MemFact         MemoryType = "fact"
)

type SessionStatus string

const (
	SessionActive    SessionStatus = "active"
	SessionCompleted SessionStatus = "completed"
	SessionAbandoned SessionStatus = "abandoned"
)

type ActionStatus string

const (
	ActionPending    ActionStatus = "pending"
	ActionBlocked    ActionStatus = "blocked"
	ActionInProgress ActionStatus = "in_progress"
	ActionDone       ActionStatus = "done"
	ActionCancelled  ActionStatus = "cancelled"
)

type LeaseStatus string

const (
	LeaseActive   LeaseStatus = "active"
	LeaseReleased LeaseStatus = "released"
	LeaseExpired  LeaseStatus = "expired"
)

type SentinelType string

const (
	SentinelWebhook   SentinelType = "webhook"
	SentinelTimer     SentinelType = "timer"
	SentinelThreshold SentinelType = "threshold"
	SentinelPattern   SentinelType = "pattern"
	SentinelApproval  SentinelType = "approval"
	SentinelCustom    SentinelType = "custom"
)

type SentinelStatus string

const (
	SentinelWatching  SentinelStatus = "watching"
	SentinelTriggered SentinelStatus = "triggered"
	SentinelCancelled SentinelStatus = "cancelled"
	SentinelExpired   SentinelStatus = "expired"
)

type CheckpointStatus string

const (
	CheckpointPending  CheckpointStatus = "pending"
	CheckpointApproved CheckpointStatus = "approved"
	CheckpointRejected CheckpointStatus = "rejected"
)

type SketchStatus string

const (
	SketchActive    SketchStatus = "active"
	SketchPromoted  SketchStatus = "promoted"
	SketchDiscarded SketchStatus = "discarded"
	SketchExpired   SketchStatus = "expired"
)

type GraphNodeType string

const (
	NodeFile         GraphNodeType = "file"
	NodeFunction     GraphNodeType = "function"
	NodeConcept      GraphNodeType = "concept"
	NodeError        GraphNodeType = "error"
	NodeDecision     GraphNodeType = "decision"
	NodePattern      GraphNodeType = "pattern"
	NodeLibrary      GraphNodeType = "library"
	NodePerson       GraphNodeType = "person"
	NodeProject      GraphNodeType = "project"
	NodePreference   GraphNodeType = "preference"
	NodeLocation     GraphNodeType = "location"
	NodeOrganization GraphNodeType = "organization"
	NodeEvent        GraphNodeType = "event"
)

type GraphEdgeType string

const (
	EdgeUses          GraphEdgeType = "uses"
	EdgeImports       GraphEdgeType = "imports"
	EdgeModifies      GraphEdgeType = "modifies"
	EdgeCauses        GraphEdgeType = "causes"
	EdgeFixes         GraphEdgeType = "fixes"
	EdgeDependsOn     GraphEdgeType = "depends_on"
	EdgeRelatedTo     GraphEdgeType = "related_to"
	EdgeWorksAt       GraphEdgeType = "works_at"
	EdgePrefers       GraphEdgeType = "prefers"
	EdgeBlockedBy     GraphEdgeType = "blocked_by"
	EdgeCausedBy      GraphEdgeType = "caused_by"
	EdgeOptimizesFor  GraphEdgeType = "optimizes_for"
	EdgeRejected      GraphEdgeType = "rejected"
	EdgeAvoids        GraphEdgeType = "avoids"
	EdgeLocatedIn     GraphEdgeType = "located_in"
	EdgeSucceededBy   GraphEdgeType = "succeeded_by"
	EdgeRequires      GraphEdgeType = "requires"
	EdgeUnlocks       GraphEdgeType = "unlocks"
	EdgeSpawnedBy     GraphEdgeType = "spawned_by"
	EdgeGatedBy       GraphEdgeType = "gated_by"
	EdgeConflictsWith GraphEdgeType = "conflicts_with"
)

type AuditAction string

const (
	AuditSessionStart    AuditAction = "session.start"
	AuditSessionEnd      AuditAction = "session.end"
	AuditObserve         AuditAction = "observation.create"
	AuditCompress        AuditAction = "observation.compress"
	AuditRemember        AuditAction = "memory.create"
	AuditForget          AuditAction = "memory.delete"
	AuditEvolve          AuditAction = "memory.evolve"
	AuditConsolidate     AuditAction = "memory.consolidate"
	AuditSearch          AuditAction = "search.execute"
	AuditContextBuild    AuditAction = "context.build"
	AuditGraphExtract    AuditAction = "graph.extract"
	AuditGraphQuery      AuditAction = "graph.query"
	AuditActionCreate    AuditAction = "action.create"
	AuditActionUpdate    AuditAction = "action.update"
	AuditLeaseAcquire    AuditAction = "lease.acquire"
	AuditLeaseRelease    AuditAction = "lease.release"
	AuditExport          AuditAction = "data.export"
	AuditImport          AuditAction = "data.import"
	AuditEvict           AuditAction = "data.evict"
	AuditSnapshotCreate  AuditAction = "snapshot.create"
	AuditSnapshotRestore AuditAction = "snapshot.restore"
	AuditMeshSync        AuditAction = "mesh.sync"
	AuditTeamShare       AuditAction = "team.share"
	AuditGovernanceDelete AuditAction = "governance.delete"
	AuditBulkDelete      AuditAction = "governance.bulk_delete"
	AuditLessonCreate    AuditAction = "lesson.create"
	AuditInsightCreate   AuditAction = "insight.create"
	AuditReflect         AuditAction = "reflect.execute"
	AuditCrystallize     AuditAction = "crystal.create"
	AuditSentinelCreate  AuditAction = "sentinel.create"
	AuditSentinelTrigger AuditAction = "sentinel.trigger"
	AuditRoutineRun      AuditAction = "routine.run"
)
