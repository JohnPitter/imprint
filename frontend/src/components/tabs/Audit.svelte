<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../../lib/api';
  import { timeAgo } from '../../lib/format';

  let entries: any[] = [];
  let loading = true;
  let offset = 0;
  const limit = 50;

  onMount(() => load());

  async function load() {
    loading = true;
    try {
      const r = await api.listAudit(limit, offset);
      entries = r.entries || [];
    } catch(e) { console.error(e); }
    loading = false;
  }

  function prev() { if (offset >= limit) { offset -= limit; load(); } }
  function next() { if (entries.length >= limit) { offset += limit; load(); } }

  $: currentPage = Math.floor(offset / limit) + 1;
  $: totalPages = entries.length < limit ? currentPage : currentPage + 1;

  const actionColors: Record<string, string> = {
    'session.start': 'badge-success', 'session.end': 'badge-info',
    'observation.create': 'badge-accent', 'memory.create': 'badge-purple',
    'memory.delete': 'badge-danger', 'search.execute': 'badge-info',
    'governance.delete': 'badge-danger', 'governance.bulk_delete': 'badge-danger',
  };
</script>

<div class="audit-container">
  <div class="audit-header">
    <div>
      <div class="gold-line"></div>
      <h3>Audit Trail</h3>
    </div>
  </div>

  {#if loading}
    <div class="loading-rows">
      {#each Array(8) as _}
        <div class="skeleton-row"></div>
      {/each}
    </div>
  {:else if entries.length === 0}
    <div class="empty-state">
      <div class="icon" style="font-size:32px; opacity:0.15">&#9670;</div>
      <p>No audit entries yet</p>
    </div>
  {:else}
    <div class="table-wrapper">
      <table>
        <thead>
          <tr>
            <th>TIME</th>
            <th>ACTION</th>
            <th>ENTITY</th>
            <th>TYPE</th>
            <th>AGENT</th>
          </tr>
        </thead>
        <tbody>
          {#each entries as e}
            <tr>
              <td class="mono td-time">{timeAgo(e.timestamp)}</td>
              <td><span class="badge {actionColors[e.action] || 'badge-accent'}">{e.action}</span></td>
              <td class="mono td-entity">{e.entityId || '\u2014'}</td>
              <td class="td-type">{e.entityType || '\u2014'}</td>
              <td class="mono td-agent">{e.agentId || '\u2014'}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    <div class="pagination">
      <button class="pagination-btn" on:click={prev} disabled={offset === 0}>\u2190 PREV</button>
      <span class="pagination-info">PAGE {currentPage} OF {totalPages}</span>
      <button class="pagination-btn" on:click={next} disabled={entries.length < limit}>NEXT \u2192</button>
    </div>
  {/if}
</div>

<style>
  .audit-container {
    display: flex;
    flex-direction: column;
    gap: 0;
  }
  .audit-header {
    margin-bottom: 20px;
  }
  .audit-header h3 {
    font-family: var(--font-display);
    font-size: 16px;
    font-weight: 600;
    letter-spacing: -0.02em;
  }

  .table-wrapper {
    border: 1px solid var(--border);
    background: var(--bg-card);
    overflow-x: auto;
  }

  table { width: 100%; border-collapse: collapse; }

  thead tr {
    border-bottom: 2px solid var(--accent);
  }
  th {
    text-align: left;
    padding: 12px 16px;
    font-family: var(--font-ui);
    font-size: 10px;
    font-weight: 700;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.1em;
    border-bottom: none;
  }
  td {
    padding: 12px 16px;
    border-bottom: 1px solid var(--border);
    font-size: 13px;
  }
  tr { transition: background 0.15s var(--ease); }
  tbody tr:hover td { background: var(--bg-hover); }

  .td-time { font-size: 11px; color: var(--text-muted); }
  .td-entity { font-size: 12px; color: var(--text-secondary); }
  .td-type { color: var(--text-secondary); font-size: 13px; }
  .td-agent { font-size: 12px; color: var(--text-muted); }

  /* Pagination */
  .pagination {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 24px;
    margin-top: 20px;
    padding-top: 16px;
  }
  .pagination-btn {
    font-family: var(--font-ui);
    font-size: 10px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.1em;
    color: var(--text-muted);
    background: none;
    border: none;
    cursor: pointer;
    padding: 6px 12px;
    transition: color 0.2s var(--ease);
  }
  .pagination-btn:hover:not(:disabled) { color: var(--accent); }
  .pagination-btn:disabled { opacity: 0.3; cursor: not-allowed; }
  .pagination-info {
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--text-muted);
    letter-spacing: 0.08em;
  }

  /* Loading */
  .loading-rows { display: flex; flex-direction: column; gap: 2px; }
  .skeleton-row {
    height: 44px;
    background: var(--bg-card);
    border: 1px solid var(--border);
    animation: pulse 1.5s ease-in-out infinite;
  }
  @keyframes pulse {
    0%, 100% { opacity: 0.4; }
    50% { opacity: 0.8; }
  }
</style>
