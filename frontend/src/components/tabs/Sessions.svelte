<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../../lib/api';
  import { timeAgo, truncate } from '../../lib/format';

  let sessions: any[] = [];
  let selected: any = null;
  let observations: any[] = [];
  let loading = true;
  let actionLoading = '';
  let actionMessage = '';

  const typeIcons: Record<string, string> = {
    file_operation: '\u{1F4C4}',
    command_execution: '\u26A1',
    search: '\u{1F50D}',
    error: '\u26A0\uFE0F',
    decision: '\u{1F914}',
    discovery: '\u{1F4A1}',
    conversation: '\u{1F4AC}',
    notification: '\u{1F514}',
    subagent_event: '\u{1F916}',
    task: '\u2611\uFE0F',
    other: '\u{1F4C4}',
  };

  const typeColors: Record<string, string> = {
    file_operation: 'badge-info',
    command_execution: 'badge-accent',
    error: 'badge-danger',
    decision: 'badge-warning',
    discovery: 'badge-success',
    search: 'badge-info',
    conversation: 'badge-purple',
  };

  function getField(o: any, ...keys: string[]): any {
    for (const k of keys) {
      if (o[k] !== undefined && o[k] !== null && o[k] !== '') return o[k];
    }
    return undefined;
  }

  function getSessionId(s: any): string {
    return s.ID || s.id || '';
  }

  function getStatus(s: any): string {
    return s.Status || s.status || 'unknown';
  }

  function statusBadgeClass(status: string): string {
    if (status === 'active') return 'badge-success';
    if (status === 'completed' || status === 'ended') return 'badge-info';
    if (status === 'error') return 'badge-danger';
    return 'badge-warning';
  }

  function formatTimestamp(ts: string): string {
    if (!ts) return '\u2014';
    try {
      return new Date(ts).toLocaleString();
    } catch {
      return ts;
    }
  }

  onMount(async () => {
    try {
      const r: any = await api.listSessions(100);
      sessions = r.sessions || [];
    } catch (e) {
      console.error(e);
    }
    loading = false;
  });

  async function selectSession(s: any) {
    selected = s;
    actionMessage = '';
    try {
      const r: any = await api.listObservations(getSessionId(s));
      observations = r.observations || [];
    } catch (e) {
      observations = [];
    }
  }

  async function endSession() {
    if (!selected) return;
    actionLoading = 'end';
    actionMessage = '';
    try {
      await api.endSession({ sessionId: getSessionId(selected) });
      actionMessage = 'Session ended successfully.';
      // Refresh session list
      const r: any = await api.listSessions(100);
      sessions = r.sessions || [];
      // Update selected with fresh data
      const fresh = sessions.find((s: any) => getSessionId(s) === getSessionId(selected));
      if (fresh) selected = fresh;
    } catch (e: any) {
      actionMessage = 'Error ending session: ' + (e.message || e);
    }
    actionLoading = '';
  }

  async function summarizeSession() {
    if (!selected) return;
    actionLoading = 'summarize';
    actionMessage = '';
    try {
      await api.summarize({ sessionId: getSessionId(selected) });
      actionMessage = 'Summarization complete.';
    } catch (e: any) {
      actionMessage = 'Summarize failed: ' + (e.message || 'This endpoint is not yet implemented.');
    }
    actionLoading = '';
  }
</script>

{#if loading}
  <p style="color:var(--text-muted)">Loading sessions...</p>
{:else if sessions.length === 0}
  <div class="empty-state">
    <div class="icon">{'\u{1F4C2}'}</div>
    <p>No sessions yet</p>
  </div>
{:else}
  <div class="ss-layout">
    <!-- Session List -->
    <div class="ss-list">
      {#each sessions as s}
        {@const sid = getSessionId(s)}
        {@const status = getStatus(s)}
        {@const isActive = selected && getSessionId(selected) === sid}
        <button
          class="ss-item"
          class:ss-item-active={isActive}
          on:click={() => selectSession(s)}
        >
          <div class="ss-item-header">
            <span class="ss-item-project">{s.Project || s.project || '\u2014'}</span>
            <span class="badge {statusBadgeClass(status)}">{status}</span>
          </div>
          <div class="ss-item-id mono">{truncate(sid, 16)}</div>
          <div class="ss-item-meta">
            <span>{s.ObservationCount || s.observationCount || 0} obs</span>
            <span>{'\u00B7'}</span>
            <span>{timeAgo(s.StartedAt || s.startedAt || s.CreatedAt || s.createdAt)}</span>
          </div>
        </button>
      {/each}
    </div>

    <!-- Detail Panel -->
    <div class="ss-detail">
      {#if selected}
        {@const sid = getSessionId(selected)}
        {@const status = getStatus(selected)}
        {@const startedAt = selected.StartedAt || selected.startedAt || selected.CreatedAt || selected.createdAt}
        {@const endedAt = selected.EndedAt || selected.endedAt}
        {@const project = selected.Project || selected.project}
        {@const workDir = selected.WorkingDir || selected.workingDir || selected.WorkDir || selected.workDir}
        {@const tags = selected.Tags || selected.tags}
        {@const obsCount = selected.ObservationCount || selected.observationCount || 0}

        <div class="card ss-info-card">
          <h3 style="margin-bottom:16px;font-size:16px">Session Details</h3>

          <div class="ss-info-grid">
            <div class="ss-info-row">
              <span class="ss-info-label">Session ID</span>
              <span class="mono ss-info-value" style="font-size:11px;word-break:break-all">{sid}</span>
            </div>
            {#if project}
              <div class="ss-info-row">
                <span class="ss-info-label">Project</span>
                <span class="ss-info-value">{project}</span>
              </div>
            {/if}
            {#if workDir}
              <div class="ss-info-row">
                <span class="ss-info-label">Working Directory</span>
                <span class="mono ss-info-value" style="font-size:11px">{workDir}</span>
              </div>
            {/if}
            <div class="ss-info-row">
              <span class="ss-info-label">Status</span>
              <span class="badge {statusBadgeClass(status)}">{status}</span>
            </div>
            {#if startedAt}
              <div class="ss-info-row">
                <span class="ss-info-label">Started</span>
                <span class="ss-info-value">{formatTimestamp(startedAt)}</span>
              </div>
            {/if}
            {#if endedAt}
              <div class="ss-info-row">
                <span class="ss-info-label">Ended</span>
                <span class="ss-info-value">{formatTimestamp(endedAt)}</span>
              </div>
            {/if}
            <div class="ss-info-row">
              <span class="ss-info-label">Observations</span>
              <span class="ss-info-value mono">{obsCount}</span>
            </div>
            {#if tags && tags.length > 0}
              <div class="ss-info-row">
                <span class="ss-info-label">Tags</span>
                <div class="ss-tags">
                  {#each tags as tag}
                    <span class="badge badge-accent">{tag}</span>
                  {/each}
                </div>
              </div>
            {/if}
          </div>

          <!-- Action Buttons -->
          <div class="ss-actions">
            {#if status === 'active'}
              <button class="btn btn-primary" on:click={endSession} disabled={actionLoading === 'end'}>
                {actionLoading === 'end' ? 'Ending...' : 'End Session'}
              </button>
            {/if}
            <button class="btn" on:click={summarizeSession} disabled={actionLoading === 'summarize'}>
              {actionLoading === 'summarize' ? 'Summarizing...' : 'Summarize'}
            </button>
          </div>

          {#if actionMessage}
            <div class="ss-action-msg" class:ss-msg-error={actionMessage.startsWith('Error') || actionMessage.startsWith('Summarize failed')}>
              {actionMessage}
            </div>
          {/if}
        </div>

        <!-- Observations -->
        <h3 style="margin:20px 0 12px;font-size:15px">Observations ({observations.length})</h3>
        {#if observations.length === 0}
          <p style="color:var(--text-muted)">No observations for this session.</p>
        {:else}
          <div class="ss-obs-list">
            {#each observations as o}
              {@const obsType = getField(o, 'Type', 'type') || 'other'}
              {@const title = getField(o, 'Title', 'title', 'ToolName', 'toolName') || '\u2014'}
              {@const importance = getField(o, 'Importance', 'importance')}
              {@const timestamp = getField(o, 'Timestamp', 'timestamp')}
              {@const narrative = getField(o, 'Narrative', 'narrative')}
              {@const facts = getField(o, 'Facts', 'facts')}
              {@const concepts = getField(o, 'Concepts', 'concepts')}
              {@const files = getField(o, 'Files', 'files')}
              <div class="card ss-obs-card">
                <div class="ss-obs-header">
                  <div class="ss-obs-title">
                    <span>{typeIcons[obsType] || '\u{1F4C4}'}</span>
                    <span class="badge {typeColors[obsType] || 'badge-info'}">{obsType.replace('_', ' ')}</span>
                    <strong style="font-size:13px">{title}</strong>
                    {#if importance}
                      <span class="mono" style="color:var(--accent);font-size:12px">{'\u2605'}{importance}</span>
                    {/if}
                  </div>
                  {#if timestamp}
                    <span style="font-size:11px;color:var(--text-muted);white-space:nowrap">{timeAgo(timestamp)}</span>
                  {/if}
                </div>
                {#if narrative}
                  <p style="font-size:13px;color:var(--text-secondary);line-height:1.5">{narrative}</p>
                {/if}
                {#if facts && facts.length > 0}
                  <ul class="ss-obs-facts">
                    {#each facts as fact}
                      <li>{fact}</li>
                    {/each}
                  </ul>
                {/if}
                {#if (concepts && concepts.length > 0) || (files && files.length > 0)}
                  <div style="display:flex;gap:4px;flex-wrap:wrap;margin-top:6px">
                    {#if concepts}
                      {#each concepts as c}
                        <span class="badge badge-accent">{c}</span>
                      {/each}
                    {/if}
                    {#if files}
                      {#each files as f}
                        <span class="badge badge-info" title={f}>{truncate(f, 36)}</span>
                      {/each}
                    {/if}
                  </div>
                {/if}
              </div>
            {/each}
          </div>
        {/if}
      {:else}
        <div class="empty-state">
          <div class="icon">{'\u{1F449}'}</div>
          <p>Select a session to view details</p>
        </div>
      {/if}
    </div>
  </div>
{/if}

<style>
  .ss-layout {
    display: grid;
    grid-template-columns: 320px 1fr;
    gap: 20px;
    height: calc(100vh - 160px);
  }
  .ss-list {
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding-right: 4px;
  }
  .ss-detail {
    overflow-y: auto;
    padding-right: 4px;
  }
  .ss-item {
    text-align: left;
    padding: 12px 14px;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--bg-card);
    cursor: pointer;
    transition: all 0.15s;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .ss-item:hover {
    border-color: var(--border-hover);
    background: var(--bg-hover);
  }
  .ss-item-active {
    border-color: var(--accent);
    background: var(--accent-muted);
  }
  .ss-item-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 8px;
  }
  .ss-item-project {
    font-weight: 600;
    font-size: 13px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .ss-item-id {
    font-size: 11px;
    color: var(--text-muted);
  }
  .ss-item-meta {
    display: flex;
    gap: 6px;
    font-size: 12px;
    color: var(--text-muted);
  }

  /* Detail panel */
  .ss-info-card {
    padding: 20px;
  }
  .ss-info-grid {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .ss-info-row {
    display: flex;
    gap: 12px;
    align-items: baseline;
  }
  .ss-info-label {
    font-size: 11px;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.5px;
    min-width: 120px;
    flex-shrink: 0;
  }
  .ss-info-value {
    color: var(--text-primary);
    font-size: 13px;
  }
  .ss-tags {
    display: flex;
    gap: 4px;
    flex-wrap: wrap;
  }
  .ss-actions {
    display: flex;
    gap: 8px;
    margin-top: 20px;
    padding-top: 16px;
    border-top: 1px solid var(--border);
  }
  .ss-action-msg {
    margin-top: 10px;
    font-size: 12px;
    color: var(--success);
    padding: 8px 12px;
    background: rgba(34, 197, 94, 0.1);
    border-radius: var(--radius);
  }
  .ss-msg-error {
    color: var(--danger);
    background: rgba(239, 68, 68, 0.1);
  }

  /* Observations list */
  .ss-obs-list {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .ss-obs-card {
    padding: 12px 14px;
  }
  .ss-obs-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 4px;
    gap: 8px;
  }
  .ss-obs-title {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-wrap: wrap;
    min-width: 0;
  }
  .ss-obs-facts {
    margin: 6px 0 2px 16px;
    padding: 0;
    font-size: 13px;
    color: var(--text-secondary);
    list-style: disc;
  }
  .ss-obs-facts li {
    margin-bottom: 2px;
  }
</style>
