<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api } from '../../lib/api';
  import { createPoller } from '../../lib/poller';

  let graphStats: any = null;
  let sessionsTotal = 0;
  let memoriesTotal = 0;
  let observationsTotal = 0;
  let concepts: { name: string; count: number }[] = [];
  let loading = true;
  let stopPoll: (() => void) | undefined;

  async function refresh(initial: boolean) {
    if (initial) loading = true;
    try {
      // All four totals come from server-side aggregation now: no client-side
      // sums over a paginated window, no concept extraction over a slice.
      const [s, m, g, c, o] = await Promise.all([
        api.listSessions(1, 0).catch(() => ({ sessions: [], total: 0 })),
        api.listMemories('', 1, 0).catch(() => ({ memories: [], total: 0 })),
        api.graphStats().catch(() => null),
        api.topConcepts(20).catch(() => ({ concepts: [] })),
        api.countObservations().catch(() => ({ total: 0 })),
      ]);
      sessionsTotal = (s as any).total ?? 0;
      memoriesTotal = (m as any).total ?? 0;
      graphStats = g;
      concepts = (c as any).concepts || [];
      observationsTotal = (o as any).total ?? 0;
    } catch(e) { console.error(e); }
    if (initial) loading = false;
  }

  onMount(() => {
    refresh(true);
    stopPoll = createPoller(() => refresh(false), 15000);
  });

  onDestroy(() => {
    stopPoll?.();
  });

  $: maxConceptCount = Math.max(1, ...concepts.map(c => c.count));

  function conceptOpacity(count: number): number {
    return 0.4 + (count / maxConceptCount) * 0.6;
  }
</script>

<div class="profile-container">
  <div class="profile-header">
    <div>
      <div class="gold-line"></div>
      <h3>Profile</h3>
    </div>
  </div>

  {#if loading}
    <div class="stats-grid">
      {#each Array(4) as _}
        <div class="skeleton-stat"></div>
      {/each}
    </div>
  {:else}
    <!-- Stats Grid -->
    <div class="stats-grid">
      <div class="stat-card">
        <div class="stat-value">{sessionsTotal}</div>
        <div class="stat-label">SESSIONS</div>
      </div>
      <div class="stat-card">
        <div class="stat-value">{observationsTotal}</div>
        <div class="stat-label">OBSERVATIONS</div>
      </div>
      <div class="stat-card">
        <div class="stat-value">{memoriesTotal}</div>
        <div class="stat-label">MEMORIES</div>
      </div>
      <div class="stat-card">
        <div class="stat-value">{graphStats?.totalNodes || 0}</div>
        <div class="stat-label">GRAPH NODES</div>
      </div>
    </div>

    <!-- Top Concepts -->
    {#if concepts.length > 0}
      <div class="concepts-card">
        <div class="concepts-header">
          <div class="gold-line"></div>
          <span class="concepts-label">TOP CONCEPTS</span>
        </div>
        <div class="concepts-cloud">
          {#each concepts as c}
            <span
              class="concept-badge"
              style="opacity: {conceptOpacity(c.count)}"
            >{c.name} <span class="concept-count">{c.count}</span></span>
          {/each}
        </div>
      </div>
    {/if}
  {/if}
</div>

<style>
  .profile-container { display: flex; flex-direction: column; }
  .profile-header { margin-bottom: 24px; }
  .profile-header h3 {
    font-family: var(--font-display);
    font-size: 16px;
    font-weight: 600;
    letter-spacing: -0.02em;
  }

  /* Stats Grid */
  .stats-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 16px;
  }
  @media (max-width: 700px) {
    .stats-grid { grid-template-columns: repeat(2, 1fr); }
  }
  .stat-card {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: 0;
    padding: 24px 20px;
    text-align: center;
    transition: border-color 0.3s var(--ease), box-shadow 0.3s var(--ease);
  }
  .stat-card:hover {
    border-color: var(--accent);
    box-shadow: var(--shadow-hover);
  }
  .stat-value {
    font-family: var(--font-display);
    font-size: 32px;
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
    margin-top: 10px;
  }

  /* Concepts */
  .concepts-card {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: 0;
    padding: 28px 24px;
    margin-top: 24px;
    transition: border-color 0.3s var(--ease), box-shadow 0.3s var(--ease);
  }
  .concepts-card:hover {
    border-color: var(--accent);
    box-shadow: var(--shadow-hover);
  }
  .concepts-header {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 20px;
  }
  .concepts-header .gold-line {
    width: 32px;
    height: 2px;
    background: var(--accent);
    margin-bottom: 0;
  }
  .concepts-label {
    font-family: var(--font-ui);
    font-size: 10px;
    font-weight: 700;
    color: var(--accent);
    text-transform: uppercase;
    letter-spacing: 0.12em;
  }
  .concepts-cloud {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
  }
  .concept-badge {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 4px 14px;
    border: 1px solid rgba(200,147,58,0.25);
    background: transparent;
    color: var(--accent);
    font-family: var(--font-ui);
    font-size: 12px;
    font-weight: 600;
    letter-spacing: 0.02em;
    border-radius: 0;
    transition: background 0.2s var(--ease), border-color 0.2s var(--ease);
  }
  .concept-badge:hover {
    background: var(--accent-muted);
    border-color: var(--accent);
  }
  .concept-count {
    font-family: var(--font-mono);
    font-size: 10px;
    color: var(--text-muted);
  }

  /* Skeleton */
  .skeleton-stat {
    height: 90px;
    background: var(--bg-card);
    border: 1px solid var(--border);
    animation: pulse 1.5s ease-in-out infinite;
  }
  @keyframes pulse {
    0%, 100% { opacity: 0.4; }
    50% { opacity: 0.8; }
  }
</style>
