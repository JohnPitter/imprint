<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api } from '../../lib/api';
  import { createPoller } from '../../lib/poller';
  import { truncate } from '../../lib/format';

  let memories: any[] = [];
  let memoriesTotal = 0;
  let filter = '';
  let loading = true;
  let offset = 0;
  const limit = 30;
  const types = ['', 'pattern', 'preference', 'architecture', 'bug', 'workflow', 'fact'];
  let stopPoll: (() => void) | undefined;

  onMount(() => {
    load(true);
    stopPoll = createPoller(() => load(false), 15000);
  });

  onDestroy(() => {
    stopPoll?.();
  });

  async function load(initial: boolean) {
    if (initial) loading = true;
    try {
      const r: any = await api.listMemories(filter, limit, offset);
      memories = r.memories || [];
      memoriesTotal = r.total ?? memories.length;
    } catch(e) { console.error(e); }
    if (initial) loading = false;
  }

  function setFilter(t: string) { filter = t; offset = 0; load(true); }
  function prev() { if (offset >= limit) { offset -= limit; load(true); } }
  function next() { if (offset + limit < memoriesTotal) { offset += limit; load(true); } }

  $: currentPage = Math.floor(offset / limit) + 1;
  $: totalPages = Math.max(1, Math.ceil(memoriesTotal / limit));

  const typeColors: Record<string, string> = {
    pattern: 'badge-info', preference: 'badge-purple', architecture: 'badge-accent',
    bug: 'badge-danger', workflow: 'badge-success', fact: 'badge-warning',
  };
</script>

<!-- Type filter bar -->
<div class="filter-bar">
  {#each types as t}
    <button
      class="filter-btn"
      class:active={filter === t}
      on:click={() => setFilter(t)}
    >{t || 'ALL'}</button>
  {/each}
</div>

{#if loading}
  <div class="loading-state">
    {#each Array(3) as _}
      <div class="skeleton-card">
        <div class="skeleton-line wide"></div>
        <div class="skeleton-line"></div>
        <div class="skeleton-line narrow"></div>
      </div>
    {/each}
  </div>
{:else if memories.length === 0}
  <div class="empty-state">
    <div class="icon">--</div>
    <p>No memories found</p>
  </div>
{:else}
  <div class="memory-list">
    {#each memories as m}
      <div class="memory-card">
        <div class="card-header">
          <div class="card-header-left">
            <span class="badge {typeColors[m.type] || 'badge-info'}">{m.type}</span>
            <h4 class="memory-title">{m.title}</h4>
          </div>
          <span class="version-label mono">v{m.version}</span>
        </div>

        <p class="memory-content">{truncate(m.content, 200)}</p>

        <div class="memory-meta">
          <div class="strength-row">
            <span class="strength-label">Strength</span>
            <div class="gauge">
              <div class="gauge-fill" style="width:{m.strength * 10}%"></div>
            </div>
            <span class="strength-val mono">{m.strength}/10</span>
          </div>

          {#if m.concepts?.length > 0}
            <div class="concepts">
              {#each (typeof m.concepts === 'string' ? JSON.parse(m.concepts) : m.concepts) as c}
                <span class="badge badge-accent">{c}</span>
              {/each}
            </div>
          {/if}
        </div>
      </div>
    {/each}
  </div>

  <div class="pagination">
    <button class="pagination-btn" on:click={prev} disabled={offset === 0}>{'\u2190'} PREV</button>
    <span class="pagination-info">PAGE {currentPage} OF {totalPages}</span>
    <button class="pagination-btn" on:click={next} disabled={offset + limit >= memoriesTotal}>NEXT {'\u2192'}</button>
  </div>
{/if}

<style>
  /* Filter bar */
  .filter-bar {
    display: flex;
    align-items: center;
    gap: 0;
    margin-bottom: 24px;
    border-bottom: 1px solid var(--border);
  }
  .filter-btn {
    background: none;
    border: none;
    border-bottom: 2px solid transparent;
    padding: 10px 16px;
    font-family: var(--font-ui);
    font-size: 11px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.1em;
    color: var(--text-muted);
    cursor: pointer;
    transition: color 0.2s var(--ease), border-color 0.2s var(--ease);
  }
  .filter-btn:hover {
    color: var(--text-primary);
  }
  .filter-btn.active {
    color: var(--accent);
    border-bottom-color: var(--accent);
  }

  /* Memory list */
  .memory-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  /* Memory card */
  .memory-card {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-left: 2px solid var(--accent);
    padding: 20px 24px;
    transition: border-color 0.3s var(--ease), box-shadow 0.3s var(--ease);
  }
  .memory-card:hover {
    border-color: var(--accent);
    box-shadow: var(--shadow-hover);
  }

  .card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 10px;
  }
  .card-header-left {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .memory-title {
    font-family: var(--font-display);
    font-size: 16px;
    font-weight: 700;
    color: var(--text-primary);
    letter-spacing: -0.02em;
    line-height: 1.3;
  }
  .version-label {
    font-size: 11px;
    color: var(--text-muted);
    flex-shrink: 0;
  }

  .memory-content {
    font-family: var(--font-body);
    font-size: 13px;
    color: var(--text-dim);
    line-height: 1.6;
    margin-bottom: 14px;
  }

  .memory-meta {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  /* Strength gauge */
  .strength-row {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .strength-label {
    font-size: 10px;
    font-family: var(--font-ui);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.1em;
    color: var(--text-muted);
    flex-shrink: 0;
  }
  .strength-val {
    font-size: 11px;
    color: var(--text-muted);
    flex-shrink: 0;
  }

  /* Concepts */
  .concepts {
    display: flex;
    gap: 4px;
    flex-wrap: wrap;
  }

  /* Loading skeleton */
  .loading-state {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .skeleton-card {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-left: 2px solid var(--border-hover);
    padding: 20px 24px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .skeleton-line {
    height: 12px;
    width: 60%;
    background: var(--bg-hover);
    animation: pulse 1.5s ease-in-out infinite;
  }
  .skeleton-line.wide { width: 80%; height: 16px; }
  .skeleton-line.narrow { width: 40%; }
  @keyframes pulse {
    0%, 100% { opacity: 0.3; }
    50% { opacity: 0.7; }
  }
</style>
