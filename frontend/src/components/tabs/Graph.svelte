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

{#if loading}
  <p style="color:var(--text-muted)">Loading...</p>
{:else if !stats}
  <div class="empty-state"><div class="icon">🕸</div><p>No graph data yet</p></div>
{:else}
  <div class="stats-grid">
    <div class="card"><div class="stat-value">{stats.totalNodes || 0}</div><div class="stat-label">Total Nodes</div></div>
    <div class="card"><div class="stat-value">{stats.totalEdges || 0}</div><div class="stat-label">Total Edges</div></div>
  </div>

  <div style="display:grid;grid-template-columns:1fr 1fr;gap:16px;margin-top:20px">
    {#if stats.nodesByType && Object.keys(stats.nodesByType).length > 0}
      <div class="card">
        <h3 style="margin-bottom:12px;font-size:15px">Nodes by Type</h3>
        <table>
          <thead><tr><th>Type</th><th>Count</th></tr></thead>
          <tbody>
            {#each Object.entries(stats.nodesByType) as [type, count]}
              <tr><td><span class="badge badge-info">{type}</span></td><td class="mono">{count}</td></tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
    {#if stats.edgesByType && Object.keys(stats.edgesByType).length > 0}
      <div class="card">
        <h3 style="margin-bottom:12px;font-size:15px">Edges by Type</h3>
        <table>
          <thead><tr><th>Type</th><th>Count</th></tr></thead>
          <tbody>
            {#each Object.entries(stats.edgesByType) as [type, count]}
              <tr><td><span class="badge badge-purple">{type}</span></td><td class="mono">{count}</td></tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </div>
{/if}

<style>
  .stats-grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(180px,1fr)); gap:16px; }
</style>
