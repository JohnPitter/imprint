<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../../lib/api';
  import { timeAgo, truncate } from '../../lib/format';

  let sessions: any[] = [];
  let observations: any[] = [];
  let loading = true;
  let selectedSessionId = '';

  // Filters
  let minImportance = 1;
  let activeTypes: Set<string> = new Set();
  let allTypesActive = true;

  // Pagination
  const PAGE_SIZE = 50;
  let currentPage = 0;

  const typeLabels: Record<string, string> = {
    file_operation: 'FILE',
    command_execution: 'CMD',
    search: 'SEARCH',
    error: 'ERROR',
    decision: 'DECISION',
    discovery: 'DISCOVERY',
    conversation: 'CONV',
    notification: 'NOTIFY',
    subagent_event: 'AGENT',
    task: 'TASK',
    other: 'OTHER',
  };

  const typeColors: Record<string, string> = {
    file_operation: 'badge-info',
    command_execution: 'badge-accent',
    error: 'badge-danger',
    decision: 'badge-warning',
    discovery: 'badge-success',
    search: 'badge-info',
    conversation: 'badge-purple',
    notification: 'badge-warning',
    subagent_event: 'badge-accent',
    task: 'badge-success',
    other: 'badge-info',
  };

  function getField(o: any, ...keys: string[]): any {
    for (const k of keys) {
      if (o[k] !== undefined && o[k] !== null && o[k] !== '') return o[k];
    }
    return undefined;
  }

  function clean(s: string | undefined | null): string {
    if (!s) return '';
    return s
      .replace(/^[\s\u2000-\u2BFF\u0080-\u00BF•·–—►▸▶→←↑↓\-]+/, '')
      .replace(/\\u[0-9a-fA-F]{4}\s?/g, '')
      .replace(/[\u2022\u2023\u2013\u2014\u2190\u2192\u2193\u2194\u25a0-\u25FF\u2600-\u26FF]+\s?/g, '')
      .trim();
  }

  onMount(async () => {
    try {
      const r: any = await api.listSessions(50);
      sessions = r.sessions || [];
      if (sessions.length > 0) {
        const firstId = sessions[0].ID || sessions[0].id;
        selectedSessionId = firstId;
        await loadObservations(firstId);
      }
    } catch (e) {
      console.error(e);
    }
    loading = false;
  });

  async function loadObservations(sessionId: string) {
    selectedSessionId = sessionId;
    currentPage = 0;
    try {
      const r: any = await api.listObservations(sessionId);
      observations = r.observations || [];
    } catch (e) {
      observations = [];
    }
    activeTypes = new Set();
    allTypesActive = true;
    minImportance = 1;
  }

  // Compute type counts from all observations
  $: typeCounts = observations.reduce((acc: Record<string, number>, o: any) => {
    const t = getField(o, 'Type', 'type') || 'other';
    acc[t] = (acc[t] || 0) + 1;
    return acc;
  }, {} as Record<string, number>);

  $: typeList = Object.entries(typeCounts).sort((a, b) => b[1] - a[1]);

  // Filtered observations
  $: filtered = observations.filter((o: any) => {
    const imp = getField(o, 'Importance', 'importance') || 0;
    if (imp < minImportance) return false;
    if (!allTypesActive && activeTypes.size > 0) {
      const t = getField(o, 'Type', 'type') || 'other';
      if (!activeTypes.has(t)) return false;
    }
    return true;
  });

  // Paginated
  $: totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  $: paginated = filtered.slice(currentPage * PAGE_SIZE, (currentPage + 1) * PAGE_SIZE);

  function toggleType(t: string) {
    allTypesActive = false;
    if (activeTypes.has(t)) {
      activeTypes.delete(t);
      if (activeTypes.size === 0) allTypesActive = true;
    } else {
      activeTypes.add(t);
    }
    activeTypes = activeTypes;
    currentPage = 0;
  }

  function selectAll() {
    allTypesActive = true;
    activeTypes = new Set();
    currentPage = 0;
  }
</script>

{#if loading}
  <div class="tl-loading">
    <span class="tl-loading-label">LOADING TIMELINE</span>
  </div>
{:else}
  <!-- Controls Bar -->
  <div class="tl-controls">
    <div class="tl-control-group">
      <label class="tl-label">SESSION</label>
      <select class="tl-select" bind:value={selectedSessionId} on:change={() => loadObservations(selectedSessionId)}>
        {#each sessions as s}
          <option value={s.ID || s.id}>
            {s.Project || s.project || truncate(s.ID || s.id, 20)} {'\u2014'} {s.ObservationCount || s.observationCount || 0} obs
          </option>
        {/each}
      </select>
    </div>

    <div class="tl-control-group">
      <label class="tl-label">MIN IMPORTANCE: {minImportance}</label>
      <input type="range" min="1" max="10" bind:value={minImportance} on:input={() => { currentPage = 0; }} class="tl-range" />
    </div>
  </div>

  <!-- Type Filter Chips -->
  {#if typeList.length > 0}
    <div class="tl-chips">
      <button
        class="tl-chip"
        class:tl-chip-active={allTypesActive}
        on:click={selectAll}
      >
        ALL <span class="tl-chip-count">{observations.length}</span>
      </button>
      {#each typeList as [type, count]}
        <button
          class="tl-chip"
          class:tl-chip-active={!allTypesActive && activeTypes.has(type)}
          on:click={() => toggleType(type)}
        >
          {typeLabels[type] || type.replace('_', ' ').toUpperCase()}
          <span class="tl-chip-count">{count}</span>
        </button>
      {/each}
    </div>
  {/if}

  <!-- Results summary -->
  <div class="tl-summary">
    Showing {paginated.length} of {filtered.length} observations
  </div>

  <!-- Observation Cards -->
  {#if paginated.length === 0}
    <div class="empty-state">
      <div class="tl-empty-icon">{'\u25A0'}</div>
      <p style="font-family:var(--font-ui);font-size:13px">No observations match the current filters</p>
    </div>
  {:else}
    <div class="tl-list">
      {#each paginated as o}
        {@const obsType = getField(o, 'Type', 'type') || 'other'}
        {@const title = getField(o, 'Title', 'title', 'ToolName', 'toolName') || '\u2014'}
        {@const importance = getField(o, 'Importance', 'importance')}
        {@const timestamp = getField(o, 'Timestamp', 'timestamp')}
        {@const narrative = getField(o, 'Narrative', 'narrative')}
        {@const facts = getField(o, 'Facts', 'facts')}
        {@const concepts = getField(o, 'Concepts', 'concepts')}
        {@const files = getField(o, 'Files', 'files')}
        <div class="tl-card">
          <div class="tl-card-header">
            <div class="tl-card-title">
              <span class="badge {typeColors[obsType] || 'badge-info'}">{typeLabels[obsType] || obsType.replace('_', ' ').toUpperCase()}</span>
              <strong class="tl-title-text">{title}</strong>
              {#if importance}
                <span class="tl-importance mono">{'\u2605'}{importance}</span>
              {/if}
            </div>
            {#if timestamp}
              <span class="tl-time mono">{timeAgo(timestamp)}</span>
            {/if}
          </div>

          {#if narrative}
            <p class="tl-narrative">{clean(narrative)}</p>
          {/if}

          {#if facts && facts.length > 0}
            <ul class="tl-facts">
              {#each facts as fact}
                {@const f = clean(fact)}
                {#if f}<li>{f}</li>{/if}
              {/each}
            </ul>
          {/if}

          {#if (concepts && concepts.length > 0) || (files && files.length > 0)}
            <div class="tl-tags">
              {#if concepts && concepts.length > 0}
                {#each concepts as c}
                  <span class="badge badge-accent">{c}</span>
                {/each}
              {/if}
              {#if files && files.length > 0}
                {#each files as f}
                  <span class="badge badge-info" title={f}>{truncate(f, 40)}</span>
                {/each}
              {/if}
            </div>
          {/if}
        </div>
      {/each}
    </div>
  {/if}

  <!-- Pagination -->
  {#if totalPages > 1}
    <div class="tl-pagination">
      <button class="tl-page-btn" disabled={currentPage === 0} on:click={() => { currentPage--; }}>
        {'\u2039'}
      </button>
      <span class="tl-page-info mono">Page {currentPage + 1} of {totalPages}</span>
      <button class="tl-page-btn" disabled={currentPage >= totalPages - 1} on:click={() => { currentPage++; }}>
        {'\u203A'}
      </button>
    </div>
  {/if}
{/if}

<style>
  /* Loading */
  .tl-loading {
    padding: 40px 24px;
  }
  .tl-loading-label {
    font-family: var(--font-ui);
    font-size: 10px;
    font-weight: 600;
    color: var(--text-muted);
    letter-spacing: 0.12em;
    animation: pulse 1.4s infinite;
  }
  @keyframes pulse { 0%, 100% { opacity: 0.3; } 50% { opacity: 1; } }

  .tl-empty-icon {
    font-size: 28px;
    color: var(--accent);
    opacity: 0.2;
    margin-bottom: 16px;
  }

  /* Controls bar */
  .tl-controls {
    display: flex;
    gap: 32px;
    align-items: flex-end;
    margin-bottom: 24px;
    padding-bottom: 20px;
    border-bottom: 1px solid var(--border);
    flex-wrap: wrap;
  }
  .tl-control-group {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .tl-label {
    font-family: var(--font-ui);
    font-size: 10px;
    font-weight: 700;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.1em;
  }
  .tl-select {
    padding: 10px 14px;
    border: 1px solid var(--border);
    background: var(--bg-secondary);
    color: var(--text-primary);
    font-family: var(--font-ui);
    font-size: 13px;
    min-width: 300px;
    transition: border-color 0.2s var(--ease);
    appearance: none;
    cursor: pointer;
  }
  .tl-select:focus {
    outline: none;
    border-color: var(--accent);
  }
  .tl-range {
    width: 200px;
    accent-color: var(--accent);
    cursor: pointer;
    height: 4px;
  }

  /* Type filter chips — text-only, no background */
  .tl-chips {
    display: flex;
    gap: 4px;
    flex-wrap: wrap;
    margin-bottom: 20px;
  }
  .tl-chip {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 8px 14px;
    border: none;
    background: transparent;
    color: var(--text-muted);
    font-family: var(--font-ui);
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    cursor: pointer;
    transition: all 0.15s var(--ease);
    border-bottom: 2px solid transparent;
  }
  .tl-chip:hover {
    color: var(--text-primary);
  }
  .tl-chip-active {
    color: var(--accent);
    border-bottom-color: var(--accent);
  }
  .tl-chip-count {
    font-family: var(--font-mono);
    font-size: 10px;
    opacity: 0.6;
  }

  /* Summary line */
  .tl-summary {
    font-family: var(--font-ui);
    font-size: 11px;
    color: var(--text-muted);
    letter-spacing: 0.04em;
    margin-bottom: 16px;
  }

  /* Observation cards */
  .tl-list {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .tl-card {
    padding: 18px 24px;
    border-left: 2px solid var(--border);
    transition: all 0.2s var(--ease);
  }
  .tl-card:hover {
    border-left-color: var(--accent);
    background: var(--bg-hover);
  }
  .tl-card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
    gap: 16px;
  }
  .tl-card-title {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-wrap: wrap;
    min-width: 0;
  }
  .tl-title-text {
    font-family: var(--font-ui);
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .tl-importance {
    color: var(--accent);
    font-size: 12px;
    flex-shrink: 0;
  }
  .tl-time {
    font-size: 11px;
    color: var(--text-muted);
    white-space: nowrap;
    flex-shrink: 0;
  }
  .tl-narrative {
    font-family: var(--font-ui);
    font-size: 13px;
    color: var(--text-secondary);
    line-height: 1.6;
    margin-bottom: 6px;
  }
  .tl-facts {
    margin: 8px 0 6px 0;
    padding: 0 0 0 18px;
    font-family: var(--font-ui);
    font-size: 13px;
    color: var(--text-secondary);
    list-style: none;
  }
  .tl-facts li {
    margin-bottom: 3px;
    position: relative;
  }
  .tl-facts li::before {
    content: '\2014';
    position: absolute;
    left: -18px;
    color: var(--text-muted);
  }
  .tl-tags {
    display: flex;
    gap: 6px;
    flex-wrap: wrap;
    margin-top: 10px;
  }

  /* Pagination — minimal */
  .tl-pagination {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 20px;
    margin-top: 24px;
    padding: 20px 0;
    border-top: 1px solid var(--border);
  }
  .tl-page-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 36px;
    height: 36px;
    border: 1px solid var(--border);
    background: transparent;
    color: var(--text-secondary);
    font-size: 18px;
    cursor: pointer;
    transition: all 0.15s var(--ease);
  }
  .tl-page-btn:hover:not(:disabled) {
    border-color: var(--accent);
    color: var(--accent);
  }
  .tl-page-btn:disabled {
    opacity: 0.3;
    cursor: not-allowed;
  }
  .tl-page-info {
    font-size: 12px;
    color: var(--text-dim);
    letter-spacing: 0.04em;
  }
</style>
