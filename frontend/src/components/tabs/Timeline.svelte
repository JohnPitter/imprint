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
    // Reset filters on session change
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
    activeTypes = activeTypes; // trigger reactivity
    currentPage = 0;
  }

  function selectAll() {
    allTypesActive = true;
    activeTypes = new Set();
    currentPage = 0;
  }
</script>

{#if loading}
  <p style="color:var(--text-muted)">Loading timeline...</p>
{:else}
  <!-- Session Selector -->
  <div class="tl-controls">
    <div class="tl-control-group">
      <label class="tl-label">Session</label>
      <select class="input" style="max-width:400px" bind:value={selectedSessionId} on:change={() => loadObservations(selectedSessionId)}>
        {#each sessions as s}
          <option value={s.ID || s.id}>
            {s.Project || s.project || truncate(s.ID || s.id, 20)} — {s.ObservationCount || s.observationCount || 0} obs
          </option>
        {/each}
      </select>
    </div>

    <!-- Importance Filter -->
    <div class="tl-control-group">
      <label class="tl-label">Min Importance: {minImportance}</label>
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
        All ({observations.length})
      </button>
      {#each typeList as [type, count]}
        <button
          class="tl-chip"
          class:tl-chip-active={!allTypesActive && activeTypes.has(type)}
          on:click={() => toggleType(type)}
        >
          <span class="badge {typeColors[type] || 'badge-info'}" style="padding:1px 5px;font-size:10px">
            {typeIcons[type] || '\u{1F4C4}'} {type.replace('_', ' ')}
          </span>
          <span class="tl-chip-count">{count}</span>
        </button>
      {/each}
    </div>
  {/if}

  <!-- Results count -->
  <div style="font-size:12px;color:var(--text-muted);margin-bottom:12px">
    Showing {paginated.length} of {filtered.length} observations (page {currentPage + 1}/{totalPages})
  </div>

  <!-- Observation Cards -->
  {#if paginated.length === 0}
    <div class="empty-state">
      <div class="icon">{'\u{1F4C5}'}</div>
      <p>No observations match the current filters</p>
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
        <div class="card tl-card">
          <div class="tl-card-header">
            <div class="tl-card-title">
              <span class="tl-type-icon">{typeIcons[obsType] || '\u{1F4C4}'}</span>
              <span class="badge {typeColors[obsType] || 'badge-info'}">{obsType.replace('_', ' ')}</span>
              <strong class="tl-title-text">{title}</strong>
              {#if importance}
                <span class="mono tl-importance">{'\u2605'}{importance}</span>
              {/if}
            </div>
            {#if timestamp}
              <span class="tl-time">{timeAgo(timestamp)}</span>
            {/if}
          </div>

          {#if narrative}
            <p class="tl-narrative">{narrative}</p>
          {/if}

          {#if facts && facts.length > 0}
            <ul class="tl-facts">
              {#each facts as fact}
                <li>{fact}</li>
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
      <button class="btn" disabled={currentPage === 0} on:click={() => { currentPage--; }}>
        {'\u2190'} Prev
      </button>
      <span class="mono" style="color:var(--text-secondary)">{currentPage + 1} / {totalPages}</span>
      <button class="btn" disabled={currentPage >= totalPages - 1} on:click={() => { currentPage++; }}>
        Next {'\u2192'}
      </button>
    </div>
  {/if}
{/if}

<style>
  .tl-controls {
    display: flex;
    gap: 24px;
    align-items: flex-end;
    margin-bottom: 16px;
    flex-wrap: wrap;
  }
  .tl-control-group {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .tl-label {
    font-size: 11px;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }
  .tl-range {
    width: 180px;
    accent-color: var(--accent);
    cursor: pointer;
  }
  .tl-chips {
    display: flex;
    gap: 6px;
    flex-wrap: wrap;
    margin-bottom: 12px;
  }
  .tl-chip {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 4px 10px;
    border: 1px solid var(--border);
    border-radius: 20px;
    background: var(--bg-card);
    color: var(--text-secondary);
    font-size: 12px;
    cursor: pointer;
    transition: all 0.15s;
  }
  .tl-chip:hover {
    border-color: var(--border-hover);
    background: var(--bg-hover);
  }
  .tl-chip-active {
    border-color: var(--accent);
    background: var(--accent-muted);
    color: var(--text-primary);
  }
  .tl-chip-count {
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--text-muted);
  }
  .tl-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .tl-card {
    padding: 14px 18px;
  }
  .tl-card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 6px;
    gap: 12px;
  }
  .tl-card-title {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
    min-width: 0;
  }
  .tl-type-icon {
    font-size: 16px;
    flex-shrink: 0;
  }
  .tl-title-text {
    font-size: 14px;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .tl-importance {
    color: var(--accent);
    flex-shrink: 0;
  }
  .tl-time {
    font-size: 11px;
    color: var(--text-muted);
    white-space: nowrap;
    flex-shrink: 0;
  }
  .tl-narrative {
    font-size: 13px;
    color: var(--text-secondary);
    line-height: 1.5;
    margin-bottom: 4px;
  }
  .tl-facts {
    margin: 6px 0 4px 16px;
    padding: 0;
    font-size: 13px;
    color: var(--text-secondary);
    list-style: disc;
  }
  .tl-facts li {
    margin-bottom: 2px;
  }
  .tl-tags {
    display: flex;
    gap: 4px;
    flex-wrap: wrap;
    margin-top: 8px;
  }
  .tl-pagination {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 16px;
    margin-top: 16px;
    padding: 12px 0;
  }
</style>
