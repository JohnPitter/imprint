<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../../lib/api';
  import { timeAgo } from '../../lib/format';

  let crystals: any[] = [];
  let loading = true;

  onMount(async () => {
    try { const r = await api.listCrystals(50); crystals = r.crystals || []; } catch(e) { console.error(e); }
    loading = false;
  });
</script>

{#if loading}
  <p style="color:var(--text-muted)">Loading...</p>
{:else if crystals.length === 0}
  <div class="empty-state"><div class="icon">💎</div><p>No crystals yet. Crystals are generated from completed action chains.</p></div>
{:else}
  {#each crystals as c}
    <div class="card" style="margin-bottom:16px">
      <p style="font-size:14px;line-height:1.6">{c.narrative}</p>
      {#if c.keyOutcomes?.length > 0}
        <div style="margin-top:12px">
          <strong style="font-size:12px;color:var(--text-muted)">KEY OUTCOMES</strong>
          <ul style="margin-top:4px;padding-left:20px">
            {#each c.keyOutcomes as o}<li style="font-size:13px;color:var(--text-secondary)">{o}</li>{/each}
          </ul>
        </div>
      {/if}
      {#if c.filesAffected?.length > 0}
        <div style="display:flex;gap:4px;margin-top:8px;flex-wrap:wrap">
          {#each c.filesAffected as f}<span class="badge badge-info">{f}</span>{/each}
        </div>
      {/if}
      <div style="margin-top:8px;font-size:11px;color:var(--text-muted)">{timeAgo(c.createdAt)}</div>
    </div>
  {/each}
{/if}
