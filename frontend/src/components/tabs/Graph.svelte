<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../../lib/api';

  let stats: any = null;
  let loading = true;

  onMount(async () => {
    try { stats = await api.graphStats(); } catch(e) { console.error(e); }
    loading = false;
  });
</script>

<div class="graph-container">
  <div class="graph-header">
    <div>
      <div class="gold-line"></div>
      <h3>Knowledge Graph</h3>
    </div>
  </div>

  {#if loading}
    <div class="stats-row">
      <div class="skeleton-stat"></div>
      <div class="skeleton-stat"></div>
    </div>
  {:else if !stats}
    <div class="empty-state">
      <div class="icon" style="font-size:32px; opacity:0.15">&#9670;</div>
      <p>No graph data yet</p>
    </div>
  {:else}
    <!-- Stats Cards -->
    <div class="stats-row">
      <div class="stat-card">
        <div class="stat-value">{stats.totalNodes || 0}</div>
        <div class="stat-label">TOTAL NODES</div>
      </div>
      <div class="stat-card">
        <div class="stat-value">{stats.totalEdges || 0}</div>
        <div class="stat-label">TOTAL EDGES</div>
      </div>
    </div>

    <!-- Breakdown Tables -->
    <div class="breakdown-grid">
      {#if stats.nodesByType && Object.keys(stats.nodesByType).length > 0}
        <div class="breakdown-card">
          <div class="breakdown-header">
            <div class="gold-line"></div>
            <span class="breakdown-title">NODES BY TYPE</span>
          </div>
          <div class="table-wrapper">
            <table>
              <thead>
                <tr>
                  <th>TYPE</th>
                  <th style="text-align:right">COUNT</th>
                </tr>
              </thead>
              <tbody>
                {#each Object.entries(stats.nodesByType) as [type, count]}
                  <tr>
                    <td><span class="type-badge type-badge-node">{type}</span></td>
                    <td class="mono td-count">{count}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        </div>
      {/if}

      {#if stats.edgesByType && Object.keys(stats.edgesByType).length > 0}
        <div class="breakdown-card">
          <div class="breakdown-header">
            <div class="gold-line"></div>
            <span class="breakdown-title">EDGES BY TYPE</span>
          </div>
          <div class="table-wrapper">
            <table>
              <thead>
                <tr>
                  <th>TYPE</th>
                  <th style="text-align:right">COUNT</th>
                </tr>
              </thead>
              <tbody>
                {#each Object.entries(stats.edgesByType) as [type, count]}
                  <tr>
                    <td><span class="type-badge type-badge-edge">{type}</span></td>
                    <td class="mono td-count">{count}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .graph-container { display: flex; flex-direction: column; }
  .graph-header { margin-bottom: 24px; }
  .graph-header h3 {
    font-family: var(--font-display);
    font-size: 16px;
    font-weight: 600;
    letter-spacing: -0.02em;
  }

  /* Stats */
  .stats-row {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 16px;
    margin-bottom: 32px;
  }
  .stat-card {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: 0;
    padding: 32px 28px;
    text-align: center;
    transition: border-color 0.3s var(--ease), box-shadow 0.3s var(--ease);
  }
  .stat-card:hover {
    border-color: var(--accent);
    box-shadow: var(--shadow-hover);
  }
  .stat-value {
    font-family: var(--font-display);
    font-size: 40px;
    font-weight: 700;
    color: var(--accent);
    letter-spacing: -0.02em;
    line-height: 1;
  }
  .stat-label {
    font-family: var(--font-ui);
    font-size: 10px;
    font-weight: 700;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.12em;
    margin-top: 12px;
  }

  /* Breakdown */
  .breakdown-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 16px;
  }
  @media (max-width: 700px) {
    .breakdown-grid { grid-template-columns: 1fr; }
  }
  .breakdown-card {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: 0;
    padding: 24px;
    transition: border-color 0.3s var(--ease), box-shadow 0.3s var(--ease);
  }
  .breakdown-card:hover {
    border-color: var(--accent);
    box-shadow: var(--shadow-hover);
  }
  .breakdown-header {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 16px;
  }
  .breakdown-header .gold-line {
    width: 24px;
    height: 2px;
    background: var(--accent);
    margin-bottom: 0;
  }
  .breakdown-title {
    font-family: var(--font-ui);
    font-size: 10px;
    font-weight: 700;
    color: var(--accent);
    text-transform: uppercase;
    letter-spacing: 0.12em;
  }

  .table-wrapper { overflow-x: auto; }
  table { width: 100%; border-collapse: collapse; }
  thead tr { border-bottom: 2px solid var(--accent); }
  th {
    text-align: left;
    padding: 8px 12px;
    font-family: var(--font-ui);
    font-size: 10px;
    font-weight: 700;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.1em;
    border-bottom: none;
  }
  td {
    padding: 10px 12px;
    border-bottom: 1px solid var(--border);
    font-size: 13px;
  }
  tr { transition: background 0.15s var(--ease); }
  tbody tr:hover td { background: var(--bg-hover); }

  .td-count {
    text-align: right;
    font-size: 13px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .type-badge {
    display: inline-flex;
    align-items: center;
    padding: 3px 10px;
    border: 1px solid transparent;
    font-family: var(--font-ui);
    font-size: 10px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    border-radius: 0;
  }
  .type-badge-node {
    background: rgba(59,130,246,0.1);
    color: var(--info);
    border-color: rgba(59,130,246,0.2);
  }
  .type-badge-edge {
    background: rgba(168,85,247,0.1);
    color: var(--purple);
    border-color: rgba(168,85,247,0.2);
  }

  /* Skeleton */
  .skeleton-stat {
    height: 110px;
    background: var(--bg-card);
    border: 1px solid var(--border);
    animation: pulse 1.5s ease-in-out infinite;
  }
  @keyframes pulse {
    0%, 100% { opacity: 0.4; }
    50% { opacity: 0.8; }
  }
</style>
