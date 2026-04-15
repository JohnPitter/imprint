<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../../lib/api';
  import { truncate } from '../../lib/format';

  let memories: any[] = [];
  let filter = '';
  let loading = true;
  const types = ['', 'pattern', 'preference', 'architecture', 'bug', 'workflow', 'fact'];

  onMount(() => load());

  async function load() {
    loading = true;
    try { const r = await api.listMemories(filter, 100); memories = r.memories || []; } catch(e) { console.error(e); }
    loading = false;
  }

  function setFilter(t: string) { filter = t; load(); }

  const typeColors: Record<string, string> = {
    pattern: 'badge-info', preference: 'badge-purple', architecture: 'badge-accent',
    bug: 'badge-danger', workflow: 'badge-success', fact: 'badge-warning',
  };
</script>

<div style="display:flex;gap:4px;margin-bottom:16px;flex-wrap:wrap">
  {#each types as t}
    <button class="btn" class:btn-primary={filter === t} on:click={() => setFilter(t)}>{t || 'All'}</button>
  {/each}
</div>

{#if loading}
  <p style="color:var(--text-muted)">Loading...</p>
{:else if memories.length === 0}
  <div class="empty-state"><div class="icon">🧠</div><p>No memories yet</p></div>
{:else}
  {#each memories as m}
    <div class="card" style="margin-bottom:10px;padding:16px">
      <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">
        <div style="display:flex;gap:8px;align-items:center">
          <span class="badge {typeColors[m.type] || 'badge-info'}">{m.type}</span>
          <strong>{m.title}</strong>
        </div>
        <div style="display:flex;align-items:center;gap:8px">
          <div class="strength-bar" title="Strength: {m.strength}/10">
            <div class="strength-fill" style="width:{m.strength * 10}%"></div>
          </div>
          <span class="mono" style="font-size:11px;color:var(--text-muted)">v{m.version}</span>
        </div>
      </div>
      <p style="font-size:13px;color:var(--text-secondary)">{truncate(m.content, 200)}</p>
      {#if m.concepts?.length > 0}
        <div style="display:flex;gap:4px;margin-top:8px;flex-wrap:wrap">
          {#each (typeof m.concepts === 'string' ? JSON.parse(m.concepts) : m.concepts) as c}
            <span class="badge badge-accent">{c}</span>
          {/each}
        </div>
      {/if}
    </div>
  {/each}
{/if}

<style>
  .strength-bar { width:60px; height:6px; background:var(--border); border-radius:3px; overflow:hidden; }
  .strength-fill { height:100%; background:var(--accent); border-radius:3px; transition:width 0.3s; }
</style>
