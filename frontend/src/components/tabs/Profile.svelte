<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../../lib/api';

  let sessions: any[] = [];
  let memories: any[] = [];
  let graphStats: any = null;
  let loading = true;

  onMount(async () => {
    try {
      const [s, m, g] = await Promise.all([
        api.listSessions(100), api.listMemories('', 100), api.graphStats().catch(() => null)
      ]);
      sessions = s.sessions || [];
      memories = m.memories || [];
      graphStats = g;
    } catch(e) { console.error(e); }
    loading = false;
  });

  $: totalObs = sessions.reduce((sum: number, s: any) => sum + (s.ObservationCount || s.observationCount || 0), 0);
  $: concepts = extractConcepts(memories);

  function extractConcepts(mems: any[]): {name:string,count:number}[] {
    const counts: Record<string,number> = {};
    for (const m of mems) {
      const c = typeof m.concepts === 'string' ? JSON.parse(m.concepts || '[]') : (m.concepts || []);
      for (const concept of c) counts[concept] = (counts[concept] || 0) + 1;
    }
    return Object.entries(counts).map(([name,count]) => ({name,count})).sort((a,b) => b.count - a.count).slice(0, 20);
  }
</script>

{#if loading}
  <p style="color:var(--text-muted)">Loading...</p>
{:else}
  <div class="stats-grid">
    <div class="card"><div class="stat-value">{sessions.length}</div><div class="stat-label">Sessions</div></div>
    <div class="card"><div class="stat-value">{totalObs}</div><div class="stat-label">Observations</div></div>
    <div class="card"><div class="stat-value">{memories.length}</div><div class="stat-label">Memories</div></div>
    <div class="card"><div class="stat-value">{graphStats?.totalNodes || 0}</div><div class="stat-label">Graph Nodes</div></div>
  </div>

  {#if concepts.length > 0}
    <div class="card" style="margin-top:20px">
      <h3 style="font-size:15px;margin-bottom:12px">Top Concepts</h3>
      <div style="display:flex;gap:6px;flex-wrap:wrap">
        {#each concepts as c}
          <span class="badge badge-accent" style="font-size:12px;padding:4px 10px">{c.name} ({c.count})</span>
        {/each}
      </div>
    </div>
  {/if}
{/if}

<style>
  .stats-grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(160px,1fr)); gap:16px; }
</style>
