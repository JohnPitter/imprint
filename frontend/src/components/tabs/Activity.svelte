<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api } from '../../lib/api';
  import { createPoller } from '../../lib/poller';
  import { timeAgo, truncate } from '../../lib/format';

  let auditEntries: any[] = [];
  let loading = true;
  let stopPoll: (() => void) | undefined;
  let heatmapDays = 365; // 30 / 90 / 365 — selectable

  let heatmap: Map<string, number> = new Map();
  let heatmapWeeks: { day: number; date: string; count: number }[][] = [];
  let maxCount = 1;

  let typeBreakdown: { type: string; count: number; pct: number }[] = [];
  let feedEntries: any[] = [];

  function toDateKey(d: Date): string {
    return d.toISOString().slice(0, 10);
  }

  function buildHeatmap(buckets: { date: string; count: number }[]) {
    const map = new Map<string, number>();
    for (const b of buckets) {
      if (!b?.date) continue;
      map.set(b.date, b.count || 0);
    }
    heatmap = map;
    maxCount = Math.max(1, ...map.values());

    const today = new Date();
    const weeks: { day: number; date: string; count: number }[][] = [];
    const start = new Date(today);
    // Align to a Sunday so columns are full weeks; cover heatmapDays back from today.
    start.setDate(start.getDate() - heatmapDays - start.getDay());

    let current = new Date(start);
    let week: { day: number; date: string; count: number }[] = [];

    while (current <= today) {
      const key = toDateKey(current);
      week.push({ day: current.getDay(), date: key, count: map.get(key) || 0 });
      if (week.length === 7) { weeks.push(week); week = []; }
      current.setDate(current.getDate() + 1);
    }
    if (week.length > 0) weeks.push(week);
    heatmapWeeks = weeks;
  }

  function buildTypeBreakdown(entries: any[]) {
    const counts = new Map<string, number>();
    for (const e of entries) {
      const t = e.action || e.Action || e.operation || e.type || 'unknown';
      counts.set(t, (counts.get(t) || 0) + 1);
    }
    const total = entries.length || 1;
    const sorted = [...counts.entries()].sort((a, b) => b[1] - a[1]).slice(0, 10);
    typeBreakdown = sorted.map(([type, count]) => ({ type, count, pct: Math.round((count / total) * 100) }));
  }

  function heatLevel(count: number): number {
    if (count === 0) return 0;
    const ratio = count / maxCount;
    if (ratio <= 0.25) return 1;
    if (ratio <= 0.5) return 2;
    if (ratio <= 0.75) return 3;
    return 4;
  }

  function typeChar(op: string): string {
    const o = (op || '').toLowerCase();
    if (o.includes('create') || o.includes('add') || o.includes('remember')) return '+';
    if (o.includes('delete') || o.includes('remove') || o.includes('forget')) return '\u00D7';
    if (o.includes('update') || o.includes('evolve') || o.includes('merge')) return '\u2192';
    if (o.includes('search') || o.includes('query')) return '\u25C6';
    if (o.includes('session') || o.includes('start') || o.includes('end')) return '\u25CF';
    if (o.includes('crystal')) return '\u25C8';
    if (o.includes('lesson')) return '\u25B8';
    return '\u25AA';
  }

  function typeBadgeClass(op: string): string {
    const o = (op || '').toLowerCase();
    if (o.includes('create') || o.includes('add') || o.includes('remember') || o.includes('start')) return 'badge-success';
    if (o.includes('delete') || o.includes('remove') || o.includes('forget')) return 'badge-danger';
    if (o.includes('update') || o.includes('evolve') || o.includes('merge')) return 'badge-warning';
    if (o.includes('search') || o.includes('query')) return 'badge-purple';
    return 'badge-info';
  }

  const monthLabels = ['JAN','FEB','MAR','APR','MAY','JUN','JUL','AUG','SEP','OCT','NOV','DEC'];

  function getMonthMarkers(weeks: typeof heatmapWeeks): { label: string; col: number }[] {
    const markers: { label: string; col: number }[] = [];
    let lastMonth = -1;
    for (let i = 0; i < weeks.length; i++) {
      const firstDay = weeks[i][0];
      if (!firstDay) continue;
      const month = new Date(firstDay.date).getMonth();
      if (month !== lastMonth) {
        markers.push({ label: monthLabels[month], col: i });
        lastMonth = month;
      }
    }
    return markers;
  }

  let feedOffset = 0;
  const feedLimit = 20;

  async function refresh() {
    try {
      const [list, hm] = await Promise.all([
        api.listAudit(200, 0) as Promise<any>,
        api.auditHeatmap(heatmapDays) as Promise<any>,
      ]);
      auditEntries = list.entries || list.audit || [];
      buildHeatmap(hm.buckets || []);
      buildTypeBreakdown(auditEntries);
      feedEntries = auditEntries.slice(feedOffset, feedOffset + feedLimit);
    } catch (e) {
      console.error('Activity refresh error:', e);
    }
    loading = false;
  }

  function setHeatmapRange(d: number) {
    if (d === heatmapDays) return;
    heatmapDays = d;
    refresh();
  }

  function feedPrev() { if (feedOffset >= feedLimit) { feedOffset -= feedLimit; feedEntries = auditEntries.slice(feedOffset, feedOffset + feedLimit); } }
  function feedNext() { if (feedOffset + feedLimit < auditEntries.length) { feedOffset += feedLimit; feedEntries = auditEntries.slice(feedOffset, feedOffset + feedLimit); } }

  $: feedPage = Math.floor(feedOffset / feedLimit) + 1;
  $: feedTotalPages = Math.max(1, Math.ceil(auditEntries.length / feedLimit));
  $: heatmapTotal = [...heatmap.values()].reduce((a, b) => a + b, 0);

  onMount(() => { refresh(); stopPoll = createPoller(refresh, 10000); });
  onDestroy(() => stopPoll?.());
</script>

{#if loading}
  <div class="loading-state">
    <div class="skeleton-block skeleton-heatmap"></div>
    <div class="skeleton-block skeleton-bars">
      {#each Array(5) as _}
        <div class="skeleton-bar"></div>
      {/each}
    </div>
  </div>
{:else}
  <!-- Activity Heatmap -->
  <div class="card heatmap-card">
    <div class="section-header">
      <div>
        <div class="gold-line"></div>
        <h3>Activity Heatmap</h3>
      </div>
      <div class="heatmap-controls">
        <div class="range-toggle">
          <button class="range-btn" class:active={heatmapDays === 30} on:click={() => setHeatmapRange(30)}>30D</button>
          <button class="range-btn" class:active={heatmapDays === 90} on:click={() => setHeatmapRange(90)}>90D</button>
          <button class="range-btn" class:active={heatmapDays === 365} on:click={() => setHeatmapRange(365)}>1Y</button>
        </div>
        <span class="refresh-indicator">AUTO-REFRESH 10S  ·  {heatmapTotal} EVENTS / {heatmapDays}D</span>
      </div>
    </div>

    <div class="heatmap-wrapper">
      <div class="heatmap-months">
        {#each getMonthMarkers(heatmapWeeks) as marker}
          <span class="month-label" style="grid-column: {marker.col + 2}">{marker.label}</span>
        {/each}
      </div>

      <div class="heatmap-container">
        <div class="heatmap-days">
          <span></span>
          <span>MON</span>
          <span></span>
          <span>WED</span>
          <span></span>
          <span>FRI</span>
          <span></span>
        </div>

        <div class="heatmap-grid" style="grid-template-columns: repeat({heatmapWeeks.length}, 1fr)">
          {#each heatmapWeeks as week}
            <div class="heatmap-col">
              {#each week as cell}
                <div
                  class="heatmap-cell level-{heatLevel(cell.count)}"
                  title="{cell.date}: {cell.count} event{cell.count !== 1 ? 's' : ''}"
                ></div>
              {/each}
            </div>
          {/each}
        </div>
      </div>

      <div class="heatmap-legend">
        <span class="legend-label">LESS</span>
        <div class="heatmap-cell level-0"></div>
        <div class="heatmap-cell level-1"></div>
        <div class="heatmap-cell level-2"></div>
        <div class="heatmap-cell level-3"></div>
        <div class="heatmap-cell level-4"></div>
        <span class="legend-label">MORE</span>
      </div>
    </div>
  </div>

  <!-- Operation Breakdown + Activity Feed -->
  <div class="content-grid">
    <!-- Operation Breakdown -->
    <div class="card">
      <div class="section-header">
        <div>
          <div class="gold-line"></div>
          <h3>Operation Breakdown</h3>
        </div>
      </div>
      {#if typeBreakdown.length > 0}
        <div class="breakdown-list">
          {#each typeBreakdown as item}
            <div class="breakdown-item">
              <div class="breakdown-header">
                <span class="badge badge-accent">{item.type}</span>
                <span class="mono breakdown-count">{item.count} <span class="breakdown-pct">({item.pct}%)</span></span>
              </div>
              <div class="breakdown-bar-bg">
                <div class="breakdown-bar-fill" style="width: {item.pct}%"></div>
              </div>
            </div>
          {/each}
        </div>
      {:else}
        <p class="no-data">No operations recorded</p>
      {/if}
    </div>

    <!-- Activity Feed -->
    <div class="card">
      <div class="section-header">
        <div>
          <div class="gold-line"></div>
          <h3>Activity Feed</h3>
        </div>
        <span class="badge badge-accent">{feedEntries.length}</span>
      </div>
      {#if feedEntries.length > 0}
        <div class="feed-list">
          {#each feedEntries as entry}
            <div class="feed-item">
              <div class="feed-content">
                <div class="feed-title">
                  <span class="feed-char">{typeChar(entry.operation || entry.Operation)}</span>
                  <span class="badge {typeBadgeClass(entry.operation || entry.Operation)}">
                    {entry.operation || entry.Operation || '\u2014'}
                  </span>
                  <span class="mono feed-entity">
                    {entry.entityType || entry.EntityType || ''}{entry.entityId || entry.EntityId ? ` #${truncate(String(entry.entityId || entry.EntityId), 12)}` : ''}
                  </span>
                </div>
                {#if entry.details || entry.Details || entry.narrative || entry.Narrative}
                  <p class="feed-narrative">{truncate(entry.details || entry.Details || entry.narrative || entry.Narrative || '', 120)}</p>
                {/if}
              </div>
              <span class="mono feed-time">{timeAgo(entry.timestamp || entry.Timestamp || entry.createdAt)}</span>
            </div>
          {/each}
        </div>
        <div class="pagination">
          <button class="pagination-btn" on:click={feedPrev} disabled={feedOffset === 0}>{'\u2190'} PREV</button>
          <span class="pagination-info">PAGE {feedPage} OF {feedTotalPages}</span>
          <button class="pagination-btn" on:click={feedNext} disabled={feedOffset + feedLimit >= auditEntries.length}>NEXT {'\u2192'}</button>
        </div>
      {:else}
        <p class="no-data">No activity yet</p>
      {/if}
    </div>
  </div>
{/if}

<style>
  .section-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    margin-bottom: 20px;
  }
  .section-header h3 {
    font-family: var(--font-display);
    font-size: 16px;
    font-weight: 600;
    letter-spacing: -0.02em;
  }
  .refresh-indicator {
    font-family: var(--font-ui);
    font-size: 9px;
    font-weight: 600;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.1em;
    margin-top: 4px;
  }
  .heatmap-controls {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: 8px;
  }
  .range-toggle {
    display: inline-flex;
    border: 1px solid var(--border);
  }
  .range-btn {
    background: transparent;
    border: none;
    border-right: 1px solid var(--border);
    padding: 4px 12px;
    font-family: var(--font-mono);
    font-size: 10px;
    font-weight: 700;
    color: var(--text-muted);
    cursor: pointer;
    transition: color 0.15s var(--ease), background 0.15s var(--ease);
    letter-spacing: 0.04em;
  }
  .range-btn:last-child { border-right: none; }
  .range-btn:hover:not(.active) { color: var(--text-primary); background: var(--bg-hover); }
  .range-btn.active {
    color: var(--accent);
    background: var(--accent-muted);
  }

  /* Heatmap */
  .heatmap-card { margin-bottom: 24px; }
  .heatmap-wrapper { overflow-x: auto; }
  .heatmap-months {
    display: grid;
    grid-template-columns: 32px repeat(52, 1fr);
    margin-bottom: 4px;
  }
  .month-label {
    font-family: var(--font-ui);
    font-size: 9px;
    font-weight: 700;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.1em;
  }
  .heatmap-container { display: flex; gap: 4px; }
  .heatmap-days {
    display: flex;
    flex-direction: column;
    gap: 2px;
    width: 32px;
    flex-shrink: 0;
  }
  .heatmap-days span {
    height: 12px;
    font-family: var(--font-ui);
    font-size: 9px;
    font-weight: 600;
    color: var(--text-muted);
    line-height: 12px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }
  .heatmap-grid { display: grid; gap: 2px; flex: 1; }
  .heatmap-col { display: flex; flex-direction: column; gap: 2px; }
  .heatmap-cell {
    width: 12px;
    height: 12px;
    border-radius: 0;
    transition: opacity 0.15s var(--ease);
  }
  .heatmap-cell:hover { opacity: 0.75; outline: 1px solid var(--accent); }

  .level-0 { background: var(--bg-hover); }
  .level-1 { background: rgba(200,147,58,0.15); }
  .level-2 { background: rgba(200,147,58,0.35); }
  .level-3 { background: rgba(200,147,58,0.6); }
  .level-4 { background: var(--accent); }

  .heatmap-legend {
    display: flex;
    align-items: center;
    gap: 4px;
    justify-content: flex-end;
    margin-top: 10px;
  }
  .legend-label {
    font-family: var(--font-ui);
    font-size: 9px;
    font-weight: 600;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }
  .heatmap-legend .heatmap-cell { width: 10px; height: 10px; }

  /* Content grid */
  .content-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 24px;
  }
  @media (max-width: 900px) { .content-grid { grid-template-columns: 1fr; } }

  /* Breakdown */
  .breakdown-list { display: flex; flex-direction: column; gap: 14px; }
  .breakdown-item { display: flex; flex-direction: column; gap: 6px; }
  .breakdown-header { display: flex; align-items: center; justify-content: space-between; }
  .breakdown-count {
    font-size: 13px;
    font-weight: 600;
    color: var(--text-primary);
  }
  .breakdown-pct {
    font-size: 11px;
    color: var(--text-muted);
    font-weight: 400;
  }
  .breakdown-bar-bg {
    width: 100%;
    height: 4px;
    background: var(--bg-hover);
    border-radius: 0;
    overflow: hidden;
  }
  .breakdown-bar-fill {
    height: 100%;
    background: var(--accent);
    border-radius: 0;
    transition: width 0.4s var(--ease-out);
    min-width: 2px;
  }

  /* Feed */
  .feed-list {
    display: flex;
    flex-direction: column;
    max-height: 500px;
    overflow-y: auto;
  }
  .feed-item {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
    padding: 12px 0;
    border-left: 2px solid var(--accent);
    padding-left: 14px;
    margin-left: 2px;
    transition: background 0.15s var(--ease);
  }
  .feed-item:hover { background: var(--bg-hover); }
  .feed-item + .feed-item { border-top: 1px solid var(--border); }
  .feed-content { flex: 1; min-width: 0; }
  .feed-title {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
  }
  .feed-char {
    font-size: 14px;
    color: var(--accent);
    font-weight: 700;
    flex-shrink: 0;
    width: 16px;
    text-align: center;
  }
  .feed-entity {
    font-size: 12px;
    color: var(--text-secondary);
  }
  .feed-narrative {
    font-size: 12px;
    color: var(--text-muted);
    margin-top: 4px;
    line-height: 1.5;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .feed-time {
    font-size: 11px;
    color: var(--text-muted);
    white-space: nowrap;
    flex-shrink: 0;
    margin-top: 2px;
  }

  .no-data {
    color: var(--text-muted);
    font-size: 13px;
    text-align: center;
    padding: 24px 0;
  }

  /* Skeletons */
  .loading-state { display: flex; flex-direction: column; gap: 24px; }
  .skeleton-block {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: 0;
  }
  .skeleton-heatmap { height: 120px; animation: pulse 1.5s ease-in-out infinite; }
  .skeleton-bars { display: flex; flex-direction: column; gap: 12px; padding: 24px; }
  .skeleton-bar {
    height: 20px;
    background: var(--bg-hover);
    border-radius: 0;
    animation: pulse 1.5s ease-in-out infinite;
  }
  .skeleton-bar:nth-child(1) { width: 85%; }
  .skeleton-bar:nth-child(2) { width: 65%; }
  .skeleton-bar:nth-child(3) { width: 50%; }
  .skeleton-bar:nth-child(4) { width: 35%; }
  .skeleton-bar:nth-child(5) { width: 20%; }

  @keyframes pulse {
    0%, 100% { opacity: 0.4; }
    50% { opacity: 0.8; }
  }
</style>
