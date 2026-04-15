<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../../lib/api';

  let actions: any[] = [];
  let frontier: any[] = [];
  let loading = true;

  onMount(async () => {
    try {
      const [a, f] = await Promise.all([api.listActions('', 100), api.frontier()]);
      actions = a.actions || [];
      frontier = f.actions || [];
    } catch(e) { console.error(e); }
    loading = false;
  });

  $: pending = actions.filter(a => a.status === 'pending');
  $: inProgress = actions.filter(a => a.status === 'in_progress');
  $: done = actions.filter(a => a.status === 'done');

  function priorityColor(p: number): string {
    if (p >= 8) return 'badge-danger';
    if (p >= 5) return 'badge-warning';
    return 'badge-info';
  }
</script>

{#if loading}
  <p style="color:var(--text-muted)">Loading...</p>
{:else}
  {#if frontier.length > 0}
    <div class="card" style="margin-bottom:20px;border-color:var(--accent)">
      <h3 style="margin-bottom:8px;font-size:14px;color:var(--accent)">Next Up (Frontier)</h3>
      {#each frontier as a}
        <div style="display:flex;gap:8px;align-items:center;padding:4px 0">
          <span class="badge {priorityColor(a.priority)}">P{a.priority}</span>
          <span>{a.title}</span>
        </div>
      {/each}
    </div>
  {/if}

  <div class="kanban">
    <div class="column">
      <h3 class="col-header">Pending ({pending.length})</h3>
      {#each pending as a}
        <div class="card action-card">
          <div style="display:flex;justify-content:space-between"><strong style="font-size:13px">{a.title}</strong><span class="badge {priorityColor(a.priority)}">P{a.priority}</span></div>
          {#if a.description}<p style="font-size:12px;color:var(--text-muted);margin-top:4px">{a.description}</p>{/if}
        </div>
      {/each}
    </div>
    <div class="column">
      <h3 class="col-header">In Progress ({inProgress.length})</h3>
      {#each inProgress as a}
        <div class="card action-card">
          <div style="display:flex;justify-content:space-between"><strong style="font-size:13px">{a.title}</strong><span class="badge {priorityColor(a.priority)}">P{a.priority}</span></div>
        </div>
      {/each}
    </div>
    <div class="column">
      <h3 class="col-header">Done ({done.length})</h3>
      {#each done as a}
        <div class="card action-card" style="opacity:0.7">
          <strong style="font-size:13px">{a.title}</strong>
        </div>
      {/each}
    </div>
  </div>

  {#if actions.length === 0}
    <div class="empty-state"><div class="icon">✅</div><p>No actions yet</p></div>
  {/if}
{/if}

<style>
  .kanban { display:grid; grid-template-columns:repeat(3,1fr); gap:16px; }
  .column { min-height:200px; }
  .col-header { font-size:13px; color:var(--text-muted); text-transform:uppercase; letter-spacing:0.5px; margin-bottom:12px; }
  .action-card { padding:12px; margin-bottom:8px; }
</style>
