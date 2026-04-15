<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../../lib/api';

  let lessons: any[] = [];
  let insights: any[] = [];
  let searchQuery = '';
  let loading = true;

  onMount(async () => {
    try {
      const [l, i] = await Promise.all([api.listLessons(50), api.listInsights(50)]);
      lessons = l.lessons || [];
      insights = i.insights || [];
    } catch(e) { console.error(e); }
    loading = false;
  });

  async function doSearch() {
    if (!searchQuery.trim()) return;
    try { const r = await api.searchLessons(searchQuery); lessons = r.lessons || []; } catch(e) { console.error(e); }
  }
</script>

{#if loading}
  <p style="color:var(--text-muted)">Loading...</p>
{:else}
  <h3 style="font-size:16px;margin-bottom:12px">Lessons</h3>
  <form on:submit|preventDefault={doSearch} style="margin-bottom:16px">
    <input class="input" bind:value={searchQuery} placeholder="Search lessons..." style="max-width:400px" />
  </form>

  {#if lessons.length === 0}
    <div class="empty-state" style="padding:30px"><p>No lessons yet</p></div>
  {:else}
    {#each lessons as l}
      <div class="card" style="margin-bottom:8px;padding:14px">
        <div style="display:flex;justify-content:space-between;align-items:center">
          <p style="font-size:13px;flex:1">{l.content}</p>
          <div style="display:flex;align-items:center;gap:8px;margin-left:12px">
            <div class="conf-bar"><div class="conf-fill" style="width:{(l.confidence || 0) * 100}%"></div></div>
            <span class="mono" style="font-size:11px;color:var(--text-muted)">{l.reinforcements || 0}x</span>
          </div>
        </div>
        {#if l.tags?.length > 0}
          <div style="display:flex;gap:4px;margin-top:6px;flex-wrap:wrap">
            {#each (typeof l.tags === 'string' ? JSON.parse(l.tags) : l.tags) as t}<span class="badge badge-accent">{t}</span>{/each}
          </div>
        {/if}
      </div>
    {/each}
  {/if}

  <h3 style="font-size:16px;margin:24px 0 12px">Insights</h3>
  {#if insights.length === 0}
    <div class="empty-state" style="padding:30px"><p>No insights yet</p></div>
  {:else}
    {#each insights as i}
      <div class="card" style="margin-bottom:8px;padding:14px">
        <strong style="font-size:14px">{i.title}</strong>
        <p style="font-size:13px;color:var(--text-secondary);margin-top:4px">{i.content}</p>
      </div>
    {/each}
  {/if}
{/if}

<style>
  .conf-bar { width:50px; height:6px; background:var(--border); border-radius:3px; overflow:hidden; }
  .conf-fill { height:100%; background:var(--success); border-radius:3px; }
</style>
