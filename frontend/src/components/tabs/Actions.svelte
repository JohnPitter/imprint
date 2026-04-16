<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../../lib/api';

  let actions: any[] = [];
  let frontier: any[] = [];
  let loading = true;
  let offset = 0;
  const limit = 30;

  onMount(() => load());

  async function load() {
    loading = true;
    try {
      const [a, f] = await Promise.all([api.listActions('', limit, offset), api.frontier()]);
      actions = a.actions || [];
      frontier = f.actions || [];
    } catch(e) { console.error(e); }
    loading = false;
  }

  function prev() { if (offset >= limit) { offset -= limit; load(); } }
  function next() { if (actions.length >= limit) { offset += limit; load(); } }

  $: currentPage = Math.floor(offset / limit) + 1;
  $: totalPages = actions.length < limit ? currentPage : currentPage + 1;

  $: pending = actions.filter(a => a.status === 'pending');
  $: inProgress = actions.filter(a => a.status === 'in_progress');
  $: done = actions.filter(a => a.status === 'done');

  function priorityBadge(p: number): string {
    if (p >= 8) return 'act-priority-high';
    if (p >= 5) return 'act-priority-med';
    return 'act-priority-low';
  }
</script>

{#if loading}
  <div class="act-loading">
    <span class="act-loading-label">LOADING ACTIONS</span>
  </div>
{:else}
  <!-- Frontier Section -->
  {#if frontier.length > 0}
    <div class="act-frontier">
      <div class="act-frontier-header">
        <span class="act-frontier-label">NEXT UP</span>
        <span class="act-frontier-count">{frontier.length}</span>
      </div>
      <div class="act-frontier-list">
        {#each frontier as a}
          <div class="act-frontier-item">
            <span class="act-priority {priorityBadge(a.priority)}">P{a.priority}</span>
            <span class="act-frontier-title">{a.title}</span>
            {#if a.description}
              <span class="act-frontier-desc">{'\u2014'} {a.description}</span>
            {/if}
          </div>
        {/each}
      </div>
    </div>
  {/if}

  <!-- Kanban -->
  {#if actions.length === 0}
    <div class="empty-state">
      <div class="act-empty-icon">{'\u25A0'}</div>
      <p style="font-family:var(--font-ui);font-size:13px">No actions yet</p>
    </div>
  {:else}
    <div class="act-kanban">
      <!-- Pending Column -->
      <div class="act-column">
        <div class="act-col-header">
          <div class="act-col-title-row">
            <span class="act-col-label">PENDING</span>
            <span class="act-col-count">{pending.length}</span>
          </div>
          <div class="act-col-underline"></div>
        </div>
        <div class="act-col-body">
          {#each pending as a}
            <div class="act-card">
              <div class="act-card-top">
                <span class="act-priority {priorityBadge(a.priority)}">P{a.priority}</span>
                <strong class="act-card-title">{a.title}</strong>
              </div>
              {#if a.description}
                <p class="act-card-desc">{a.description}</p>
              {/if}
            </div>
          {/each}
          {#if pending.length === 0}
            <div class="act-col-empty">{'\u2014'}</div>
          {/if}
        </div>
      </div>

      <!-- Divider -->
      <div class="act-divider"></div>

      <!-- In Progress Column -->
      <div class="act-column">
        <div class="act-col-header">
          <div class="act-col-title-row">
            <span class="act-col-label">IN PROGRESS</span>
            <span class="act-col-count">{inProgress.length}</span>
          </div>
          <div class="act-col-underline"></div>
        </div>
        <div class="act-col-body">
          {#each inProgress as a}
            <div class="act-card">
              <div class="act-card-top">
                <span class="act-priority {priorityBadge(a.priority)}">P{a.priority}</span>
                <strong class="act-card-title">{a.title}</strong>
              </div>
              {#if a.description}
                <p class="act-card-desc">{a.description}</p>
              {/if}
            </div>
          {/each}
          {#if inProgress.length === 0}
            <div class="act-col-empty">{'\u2014'}</div>
          {/if}
        </div>
      </div>

      <!-- Divider -->
      <div class="act-divider"></div>

      <!-- Done Column -->
      <div class="act-column">
        <div class="act-col-header">
          <div class="act-col-title-row">
            <span class="act-col-label">DONE</span>
            <span class="act-col-count">{done.length}</span>
          </div>
          <div class="act-col-underline"></div>
        </div>
        <div class="act-col-body">
          {#each done as a}
            <div class="act-card act-card-done">
              <div class="act-card-top">
                <strong class="act-card-title">{a.title}</strong>
              </div>
              {#if a.description}
                <p class="act-card-desc">{a.description}</p>
              {/if}
            </div>
          {/each}
          {#if done.length === 0}
            <div class="act-col-empty">{'\u2014'}</div>
          {/if}
        </div>
      </div>
    </div>

    <div class="pagination">
      <button class="pagination-btn" on:click={prev} disabled={offset === 0}>{'\u2190'} PREV</button>
      <span class="pagination-info">PAGE {currentPage} OF {totalPages}</span>
      <button class="pagination-btn" on:click={next} disabled={actions.length < limit}>NEXT {'\u2192'}</button>
    </div>
  {/if}
{/if}

<style>
  /* Loading */
  .act-loading {
    padding: 40px 24px;
  }
  .act-loading-label {
    font-family: var(--font-ui);
    font-size: 10px;
    font-weight: 600;
    color: var(--text-muted);
    letter-spacing: 0.12em;
    animation: pulse 1.4s infinite;
  }
  @keyframes pulse { 0%, 100% { opacity: 0.3; } 50% { opacity: 1; } }

  .act-empty-icon {
    font-size: 28px;
    color: var(--accent);
    opacity: 0.2;
    margin-bottom: 16px;
  }

  /* Frontier */
  .act-frontier {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-top: 2px solid var(--accent);
    padding: 24px;
    margin-bottom: 28px;
    transition: box-shadow 0.3s var(--ease);
  }
  .act-frontier:hover {
    box-shadow: var(--shadow-hover);
  }
  .act-frontier-header {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 16px;
  }
  .act-frontier-label {
    font-family: var(--font-ui);
    font-size: 10px;
    font-weight: 700;
    color: var(--accent);
    text-transform: uppercase;
    letter-spacing: 0.12em;
  }
  .act-frontier-count {
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--text-dim);
  }
  .act-frontier-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .act-frontier-item {
    display: flex;
    align-items: baseline;
    gap: 10px;
    padding: 8px 0;
    border-bottom: 1px solid var(--border);
  }
  .act-frontier-item:last-child {
    border-bottom: none;
  }
  .act-frontier-title {
    font-family: var(--font-ui);
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
  }
  .act-frontier-desc {
    font-family: var(--font-ui);
    font-size: 12px;
    color: var(--text-muted);
  }

  /* Priority badges */
  .act-priority {
    display: inline-flex;
    align-items: center;
    padding: 2px 8px;
    font-family: var(--font-mono);
    font-size: 11px;
    font-weight: 600;
    border: 1px solid transparent;
    flex-shrink: 0;
  }
  .act-priority-high {
    color: var(--accent);
    border-color: rgba(200, 147, 58, 0.3);
    background: rgba(200, 147, 58, 0.08);
  }
  .act-priority-med {
    color: var(--text-dim);
    border-color: var(--border);
    background: transparent;
  }
  .act-priority-low {
    color: var(--text-muted);
    border-color: var(--border);
    background: transparent;
  }

  /* Kanban */
  .act-kanban {
    display: grid;
    grid-template-columns: 1fr auto 1fr auto 1fr;
    gap: 0;
    min-height: 300px;
  }
  .act-divider {
    width: 1px;
    background: var(--border);
  }

  .act-column {
    min-height: 200px;
    display: flex;
    flex-direction: column;
  }
  .act-col-header {
    padding: 0 24px 16px;
  }
  .act-col-title-row {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 8px;
  }
  .act-col-label {
    font-family: var(--font-ui);
    font-size: 10px;
    font-weight: 700;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.1em;
  }
  .act-col-count {
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--text-dim);
  }
  .act-col-underline {
    width: 24px;
    height: 2px;
    background: var(--accent);
  }

  .act-col-body {
    padding: 0 24px;
    display: flex;
    flex-direction: column;
    gap: 6px;
    flex: 1;
  }
  .act-col-empty {
    color: var(--text-muted);
    font-size: 13px;
    padding: 12px 0;
    opacity: 0.4;
  }

  /* Action cards */
  .act-card {
    padding: 16px 18px;
    border: 1px solid var(--border);
    background: var(--bg-card);
    transition: all 0.2s var(--ease);
  }
  .act-card:hover {
    border-color: var(--accent);
    box-shadow: var(--shadow-hover);
  }
  .act-card-done {
    opacity: 0.5;
  }
  .act-card-done:hover {
    opacity: 0.8;
  }
  .act-card-top {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .act-card-title {
    font-family: var(--font-ui);
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
  }
  .act-card-desc {
    font-family: var(--font-ui);
    font-size: 13px;
    color: var(--text-muted);
    margin-top: 6px;
    line-height: 1.5;
  }
</style>
