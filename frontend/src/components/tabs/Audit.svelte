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
    try { const r = await api.listAudit(limit, offset); entries = r.entries || []; } catch(e) { console.error(e); }
    loading = false;
  }

  function prev() { if (offset >= limit) { offset -= limit; load(); } }
  function next() { if (entries.length >= limit) { offset += limit; load(); } }

  const actionColors: Record<string, string> = {
    'session.start': 'badge-success', 'session.end': 'badge-info',
    'observation.create': 'badge-accent', 'memory.create': 'badge-purple',
    'memory.delete': 'badge-danger', 'search.execute': 'badge-info',
    'governance.delete': 'badge-danger', 'governance.bulk_delete': 'badge-danger',
  };
</script>

{#if loading}
  <p style="color:var(--text-muted)">Loading...</p>
{:else if entries.length === 0}
  <div class="empty-state"><div class="icon">📋</div><p>No audit entries yet</p></div>
{:else}
  <table>
    <thead><tr><th>Time</th><th>Action</th><th>Entity</th><th>Type</th><th>Agent</th></tr></thead>
    <tbody>
      {#each entries as e}
        <tr>
          <td style="color:var(--text-muted);font-size:12px">{timeAgo(e.timestamp)}</td>
          <td><span class="badge {actionColors[e.action] || 'badge-info'}">{e.action}</span></td>
          <td class="mono" style="font-size:12px">{e.entityId || '—'}</td>
          <td style="color:var(--text-secondary)">{e.entityType || '—'}</td>
          <td class="mono" style="font-size:12px">{e.agentId || '—'}</td>
        </tr>
      {/each}
    </tbody>
  </table>

  <div style="display:flex;gap:8px;justify-content:center;margin-top:16px">
    <button class="btn" on:click={prev} disabled={offset === 0}>Prev</button>
    <span style="padding:8px;color:var(--text-muted);font-size:13px">Page {Math.floor(offset/limit) + 1}</span>
    <button class="btn" on:click={next} disabled={entries.length < limit}>Next</button>
  </div>
{/if}
