<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api } from '../../lib/api';
  import { timeAgo, truncate } from '../../lib/format';
  import { createPoller } from '../../lib/poller';
  import { typeLabels, typeColors, getField, clean } from '../../lib/observations';

  let sessions: any[] = $state([]);
  let sessionsTotal = $state(0);
  let selected: any = $state(null);
  let observations: any[] = $state([]);
  let loading = $state(true);
  let actionLoading = $state('');
  let actionMessage = $state('');
  let offset = $state(0);
  const limit = 30;
  let stopPoll: (() => void) | undefined;

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

  onMount(() => {
    loadSessions(true);
    stopPoll = createPoller(() => loadSessions(false), 10000);
  });

  onDestroy(() => stopPoll?.());

  async function loadSessions(initial: boolean) {
    if (initial) loading = true;
    try {
      const r: any = await api.listSessions(limit, offset);
      sessions = r.sessions || [];
      sessionsTotal = r.total ?? sessions.length;
      // Re-sync the selected session with whatever the server now reports for it
      // so polling refreshes obs counts/status while the user keeps the panel open.
      if (selected) {
        const sid = getSessionId(selected);
        const fresh = sessions.find((s: any) => getSessionId(s) === sid);
        if (fresh) selected = fresh;
      }
    } catch (e) {
      console.error(e);
    }
    if (initial) loading = false;
  }

  function prevPage() { if (offset >= limit) { offset -= limit; loadSessions(true); } }
  function nextPage() { if (offset + limit < sessionsTotal) { offset += limit; loadSessions(true); } }

  let currentPage = $derived(Math.floor(offset / limit) + 1);
  let totalPages = $derived(Math.max(1, Math.ceil(sessionsTotal / limit)));

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
      const r: any = await api.listSessions(limit, offset);
      sessions = r.sessions || [];
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
  <div class="ss-loading">
    <span class="ss-loading-text">Loading sessions</span>
    <span class="ss-loading-dots">...</span>
  </div>
{:else if sessions.length === 0}
  <div class="empty-state">
    <div class="ss-empty-icon">{'\u25C6'}</div>
    <p style="font-family:var(--font-ui);font-size:13px">No sessions recorded yet</p>
  </div>
{:else}
  <div class="ss-layout">
    <!-- Left: Session List -->
    <div class="ss-list">
      <div class="ss-list-header">
        <span class="ss-list-label">SESSIONS</span>
        <span class="ss-list-count">{sessionsTotal}</span>
      </div>
      <div class="ss-list-scroll">
        {#each sessions as s}
          {@const sid = getSessionId(s)}
          {@const status = getStatus(s)}
          {@const isActive = selected && getSessionId(selected) === sid}
          <button
            class="ss-item"
            class:ss-item-active={isActive}
            onclick={() => selectSession(s)}
          >
            <div class="ss-item-top">
              <span class="ss-item-project">{s.Project || s.project || '\u2014'}</span>
              <span class="badge {statusBadgeClass(status)}">{status}</span>
            </div>
            <span class="ss-item-id">{truncate(sid, 20)}</span>
            <div class="ss-item-meta">
              <span>{s.ObservationCount || s.observationCount || 0} obs</span>
              <span class="ss-meta-sep">{'\u00B7'}</span>
              <span>{timeAgo(s.StartedAt || s.startedAt || s.CreatedAt || s.createdAt)}</span>
            </div>
          </button>
        {/each}
      </div>
      <div class="ss-pagination">
        <button class="pagination-btn" onclick={prevPage} disabled={offset === 0}>{'\u2190'} PREV</button>
        <span class="pagination-info">PAGE {currentPage} OF {totalPages}</span>
        <button class="pagination-btn" onclick={nextPage} disabled={offset + limit >= sessionsTotal}>NEXT {'\u2192'}</button>
      </div>
    </div>

    <!-- Right: Detail Panel -->
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

        <div class="ss-detail-card">
          <div class="ss-detail-card-header">
            <span class="ss-detail-title">SESSION DETAILS</span>
          </div>

          <div class="ss-info-grid">
            <div class="ss-info-row">
              <span class="ss-info-label">SESSION ID</span>
              <span class="ss-info-value ss-mono-small">{sid}</span>
            </div>
            {#if project}
              <div class="ss-info-row">
                <span class="ss-info-label">PROJECT</span>
                <span class="ss-info-value">{project}</span>
              </div>
            {/if}
            {#if workDir}
              <div class="ss-info-row">
                <span class="ss-info-label">WORKING DIR</span>
                <span class="ss-info-value ss-mono-small">{workDir}</span>
              </div>
            {/if}
            <div class="ss-info-row">
              <span class="ss-info-label">STATUS</span>
              <span class="badge {statusBadgeClass(status)}">{status}</span>
            </div>
            {#if startedAt}
              <div class="ss-info-row">
                <span class="ss-info-label">STARTED</span>
                <span class="ss-info-value">{formatTimestamp(startedAt)}</span>
              </div>
            {/if}
            {#if endedAt}
              <div class="ss-info-row">
                <span class="ss-info-label">ENDED</span>
                <span class="ss-info-value">{formatTimestamp(endedAt)}</span>
              </div>
            {/if}
            <div class="ss-info-row">
              <span class="ss-info-label">OBSERVATIONS</span>
              <span class="ss-info-value mono">{obsCount}</span>
            </div>
            {#if tags && tags.length > 0}
              <div class="ss-info-row">
                <span class="ss-info-label">TAGS</span>
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
              <button class="ss-btn-outlined" onclick={endSession} disabled={actionLoading === 'end'}>
                {actionLoading === 'end' ? 'Ending...' : 'End Session'}
              </button>
            {/if}
            <button class="ss-btn-outlined" onclick={summarizeSession} disabled={actionLoading === 'summarize'}>
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
        <div class="ss-obs-section">
          <div class="ss-obs-header-bar">
            <span class="ss-obs-label">OBSERVATIONS</span>
            <span class="ss-obs-count">{observations.length}</span>
          </div>
          <div class="ss-obs-divider"></div>

          {#if observations.length === 0}
            <p class="ss-obs-empty">No observations for this session.</p>
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
                <div class="ss-obs-card">
                  <div class="ss-obs-top">
                    <div class="ss-obs-title">
                      <span class="badge {typeColors[obsType] || 'badge-info'}">{typeLabels[obsType] || obsType.replace('_', ' ').toUpperCase()}</span>
                      <strong class="ss-obs-name">{clean(title)}</strong>
                      {#if importance}
                        <span class="ss-obs-importance mono">{'\u2605'}{importance}</span>
                      {/if}
                    </div>
                    {#if timestamp}
                      <span class="ss-obs-time mono">{timeAgo(timestamp)}</span>
                    {/if}
                  </div>
                  {#if narrative}
                    <p class="ss-obs-narrative">{clean(narrative)}</p>
                  {/if}
                  {#if facts && facts.length > 0}
                    <ul class="ss-obs-facts">
                      {#each facts as fact}
                        {@const f = clean(fact)}
                        {#if f}<li>{f}</li>{/if}
                      {/each}
                    </ul>
                  {/if}
                  {#if (concepts && concepts.length > 0) || (files && files.length > 0)}
                    <div class="ss-obs-badges">
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
        </div>
      {:else}
        <div class="empty-state">
          <div class="ss-empty-icon">{'\u25C6'}</div>
          <p style="font-family:var(--font-ui);font-size:13px;color:var(--text-muted)">Select a session to view details</p>
        </div>
      {/if}
    </div>
  </div>
{/if}

<style>
  /* Layout */
  .ss-layout {
    display: grid;
    grid-template-columns: 320px 1fr;
    gap: 0;
    height: calc(100vh - 160px);
  }

  /* Loading */
  .ss-loading {
    padding: 40px 24px;
    color: var(--text-muted);
    font-family: var(--font-ui);
    font-size: 13px;
  }
  .ss-loading-text { letter-spacing: 0.08em; text-transform: uppercase; font-size: 10px; }
  .ss-loading-dots { animation: pulse 1.2s infinite; }
  @keyframes pulse { 0%, 100% { opacity: 0.3; } 50% { opacity: 1; } }

  .ss-empty-icon {
    font-size: 32px;
    color: var(--accent);
    opacity: 0.3;
    margin-bottom: 16px;
  }

  /* Left panel: session list */
  .ss-list {
    border-right: 1px solid var(--border);
    display: flex;
    flex-direction: column;
  }
  .ss-list-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 20px 24px 16px;
    border-bottom: 1px solid var(--border);
  }
  .ss-list-label {
    font-family: var(--font-ui);
    font-size: 10px;
    font-weight: 700;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.12em;
  }
  .ss-list-count {
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--text-dim);
  }
  .ss-pagination {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 16px;
    padding: 12px 24px;
    border-top: 1px solid var(--border);
  }

  .ss-list-scroll {
    overflow-y: auto;
    flex: 1;
  }

  .ss-item {
    text-align: left;
    width: 100%;
    padding: 16px 24px;
    border: none;
    border-bottom: 1px solid var(--border);
    border-left: 3px solid transparent;
    background: transparent;
    cursor: pointer;
    transition: all 0.15s var(--ease);
    display: flex;
    flex-direction: column;
    gap: 4px;
    color: var(--text-primary);
  }
  .ss-item:hover {
    background: var(--bg-hover);
  }
  .ss-item-active {
    border-left-color: var(--accent);
    background: var(--accent-muted);
  }
  .ss-item-top {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 8px;
  }
  .ss-item-project {
    font-family: var(--font-ui);
    font-weight: 600;
    font-size: 14px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .ss-item-id {
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--text-muted);
  }
  .ss-item-meta {
    display: flex;
    gap: 6px;
    font-size: 12px;
    color: var(--text-muted);
    font-family: var(--font-ui);
  }
  .ss-meta-sep { opacity: 0.4; }

  /* Right panel: detail */
  .ss-detail {
    overflow-y: auto;
    padding: 24px;
  }

  .ss-detail-card {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-top: 2px solid var(--accent);
    padding: 24px;
    transition: box-shadow 0.3s var(--ease);
  }
  .ss-detail-card:hover {
    box-shadow: var(--shadow-hover);
  }
  .ss-detail-card-header {
    margin-bottom: 20px;
  }
  .ss-detail-title {
    font-family: var(--font-ui);
    font-size: 10px;
    font-weight: 700;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.12em;
  }

  .ss-info-grid {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  .ss-info-row {
    display: grid;
    grid-template-columns: 140px 1fr;
    gap: 16px;
    align-items: baseline;
  }
  .ss-info-label {
    font-family: var(--font-ui);
    font-size: 10px;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.1em;
    font-weight: 600;
  }
  .ss-info-value {
    color: var(--text-primary);
    font-family: var(--font-ui);
    font-size: 13px;
  }
  .ss-mono-small {
    font-family: var(--font-mono);
    font-size: 11px;
    word-break: break-all;
    color: var(--text-dim);
  }
  .ss-tags {
    display: flex;
    gap: 6px;
    flex-wrap: wrap;
  }

  /* Action buttons — outlined, no fill */
  .ss-actions {
    display: flex;
    gap: 10px;
    margin-top: 24px;
    padding-top: 20px;
    border-top: 1px solid var(--border);
  }
  .ss-btn-outlined {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 10px 20px;
    border: 1px solid var(--border);
    background: transparent;
    color: var(--text-secondary);
    font-family: var(--font-ui);
    font-size: 12px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    cursor: pointer;
    transition: all 0.2s var(--ease);
  }
  .ss-btn-outlined:hover {
    border-color: var(--accent);
    color: var(--accent);
    box-shadow: var(--shadow-hover);
  }
  .ss-btn-outlined:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .ss-action-msg {
    margin-top: 12px;
    font-family: var(--font-ui);
    font-size: 12px;
    color: var(--success);
    padding: 10px 14px;
    background: rgba(34, 197, 94, 0.06);
    border: 1px solid rgba(34, 197, 94, 0.15);
  }
  .ss-msg-error {
    color: var(--danger);
    background: rgba(239, 68, 68, 0.06);
    border-color: rgba(239, 68, 68, 0.15);
  }

  /* Observations section */
  .ss-obs-section {
    margin-top: 32px;
  }
  .ss-obs-header-bar {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 8px;
  }
  .ss-obs-label {
    font-family: var(--font-ui);
    font-size: 10px;
    font-weight: 700;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.12em;
  }
  .ss-obs-count {
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--text-dim);
  }
  .ss-obs-divider {
    width: 40px;
    height: 2px;
    background: var(--accent);
    margin-bottom: 20px;
  }
  .ss-obs-empty {
    color: var(--text-muted);
    font-family: var(--font-ui);
    font-size: 13px;
    padding: 20px 0;
  }

  .ss-obs-list {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .ss-obs-card {
    padding: 16px 20px;
    border-left: 2px solid var(--border);
    transition: border-color 0.2s var(--ease);
  }
  .ss-obs-card:hover {
    border-left-color: var(--accent);
    background: var(--bg-hover);
  }
  .ss-obs-top {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 6px;
    gap: 12px;
  }
  .ss-obs-title {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
    min-width: 0;
  }
  .ss-obs-name {
    font-family: var(--font-ui);
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
  }
  .ss-obs-importance {
    color: var(--accent);
    font-size: 12px;
    flex-shrink: 0;
  }
  .ss-obs-time {
    font-size: 11px;
    color: var(--text-muted);
    white-space: nowrap;
    flex-shrink: 0;
  }
  .ss-obs-narrative {
    font-family: var(--font-ui);
    font-size: 13px;
    color: var(--text-secondary);
    line-height: 1.6;
    margin-bottom: 6px;
  }
  .ss-obs-facts {
    margin: 8px 0 6px 0;
    padding: 0 0 0 18px;
    font-family: var(--font-ui);
    font-size: 13px;
    color: var(--text-secondary);
    list-style: none;
  }
  .ss-obs-facts li {
    margin-bottom: 3px;
    position: relative;
  }
  .ss-obs-facts li::before {
    content: '\2014';
    position: absolute;
    left: -18px;
    color: var(--text-muted);
  }
  .ss-obs-badges {
    display: flex;
    gap: 6px;
    flex-wrap: wrap;
    margin-top: 8px;
  }
</style>
