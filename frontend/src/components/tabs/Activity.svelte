<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api } from '../../lib/api';
  import { timeAgo, truncate } from '../../lib/format';

  let auditEntries: any[] = [];
  let loading = true;
  let interval: ReturnType<typeof setInterval>;

  // Heatmap data: map of "YYYY-MM-DD" -> count
  let heatmap: Map<string, number> = new Map();
  let heatmapWeeks: { day: number; date: string; count: number }[][] = [];
  let maxCount = 1;

  // Type breakdown
  let typeBreakdown: { type: string; count: number; pct: number }[] = [];

  // Activity feed (last 20)
  let feedEntries: any[] = [];

  function toDateKey(d: Date): string {
    return d.toISOString().slice(0, 10);
  }

  function buildHeatmap(entries: any[]) {
    const map = new Map<string, number>();

    for (const e of entries) {
      const ts = e.timestamp || e.Timestamp || e.createdAt;
      if (!ts) continue;
      const key = toDateKey(new Date(ts));
      map.set(key, (map.get(key) || 0) + 1);
    }

    heatmap = map;
    maxCount = Math.max(1, ...map.values());

    // Build 52 weeks x 7 days grid (ending today)
    const today = new Date();
    const weeks: { day: number; date: string; count: number }[][] = [];

    // Find the start: 52 weeks ago, aligned to Sunday
    const start = new Date(today);
    start.setDate(start.getDate() - (52 * 7) - start.getDay());

    let current = new Date(start);
    let week: { day: number; date: string; count: number }[] = [];

    while (current <= today) {
      const key = toDateKey(current);
      week.push({
        day: current.getDay(),
        date: key,
        count: map.get(key) || 0,
      });
      if (week.length === 7) {
        weeks.push(week);
        week = [];
      }
      current.setDate(current.getDate() + 1);
    }
    if (week.length > 0) {
      weeks.push(week);
    }

    heatmapWeeks = weeks;
  }

  function buildTypeBreakdown(entries: any[]) {
    const counts = new Map<string, number>();
    for (const e of entries) {
      const t = e.operation || e.Operation || e.type || e.Type || 'unknown';
      counts.set(t, (counts.get(t) || 0) + 1);
    }

    const total = entries.length || 1;
    const sorted = [...counts.entries()]
      .sort((a, b) => b[1] - a[1])
      .slice(0, 10);

    typeBreakdown = sorted.map(([type, count]) => ({
      type,
      count,
      pct: Math.round((count / total) * 100),
    }));
  }

  function heatLevel(count: number): number {
    if (count === 0) return 0;
    const ratio = count / maxCount;
    if (ratio <= 0.25) return 1;
    if (ratio <= 0.5) return 2;
    if (ratio <= 0.75) return 3;
    return 4;
  }

  function typeIcon(op: string): string {
    const o = (op || '').toLowerCase();
    if (o.includes('create') || o.includes('add') || o.includes('remember')) return '➕';
    if (o.includes('delete') || o.includes('remove') || o.includes('forget')) return '🗑️';
    if (o.includes('update') || o.includes('evolve') || o.includes('merge')) return '🔄';
    if (o.includes('search') || o.includes('query')) return '🔍';
    if (o.includes('session') || o.includes('start') || o.includes('end')) return '📋';
    if (o.includes('crystal')) return '💎';
    if (o.includes('lesson')) return '📖';
    return '⚡';
  }

  function typeBadgeClass(op: string): string {
    const o = (op || '').toLowerCase();
    if (o.includes('create') || o.includes('add') || o.includes('remember') || o.includes('start')) return 'badge-success';
    if (o.includes('delete') || o.includes('remove') || o.includes('forget')) return 'badge-danger';
    if (o.includes('update') || o.includes('evolve') || o.includes('merge')) return 'badge-warning';
    if (o.includes('search') || o.includes('query')) return 'badge-purple';
    return 'badge-info';
  }

  const monthLabels = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec'];

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

  async function refresh() {
    try {
      const result = await api.listAudit(200, 0) as any;
      auditEntries = result.entries || result.audit || [];

      buildHeatmap(auditEntries);
      buildTypeBreakdown(auditEntries);
      feedEntries = auditEntries.slice(0, 20);
    } catch (e) {
      console.error('Activity refresh error:', e);
    }
    loading = false;
  }

  onMount(() => {
    refresh();
    interval = setInterval(refresh, 10000);
  });
  onDestroy(() => clearInterval(interval));
</script>

{#if loading}
  <div class="loading-state">
    <div class="skeleton-heatmap"></div>
    <div class="skeleton-bars">
      {#each Array(5) as _}
        <div class="skeleton-bar"></div>
      {/each}
    </div>
  </div>
{:else}
  <!-- Activity Heatmap -->
  <div class="card heatmap-card">
    <div class="section-header">
      <h3>Activity Heatmap</h3>
      <span class="refresh-info mono">Auto-refresh 10s · {auditEntries.length} events loaded</span>
    </div>

    <div class="heatmap-wrapper">
      <!-- Month labels -->
      <div class="heatmap-months">
        {#each getMonthMarkers(heatmapWeeks) as marker}
          <span class="month-label" style="grid-column: {marker.col + 2}">{marker.label}</span>
        {/each}
      </div>

      <div class="heatmap-container">
        <!-- Day labels -->
        <div class="heatmap-days">
          <span></span>
          <span>Mon</span>
          <span></span>
          <span>Wed</span>
          <span></span>
          <span>Fri</span>
          <span></span>
        </div>

        <!-- Grid -->
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

      <!-- Legend -->
      <div class="heatmap-legend">
        <span class="legend-label">Less</span>
        <div class="heatmap-cell level-0"></div>
        <div class="heatmap-cell level-1"></div>
        <div class="heatmap-cell level-2"></div>
        <div class="heatmap-cell level-3"></div>
        <div class="heatmap-cell level-4"></div>
        <span class="legend-label">More</span>
      </div>
    </div>
  </div>

  <!-- Type Breakdown + Activity Feed side by side -->
  <div class="content-grid">
    <!-- Type Breakdown -->
    <div class="card">
      <div class="section-header">
        <h3>Operation Breakdown</h3>
      </div>
      {#if typeBreakdown.length > 0}
        <div class="breakdown-list">
          {#each typeBreakdown as item}
            <div class="breakdown-item">
              <div class="breakdown-header">
                <span class="badge {typeBadgeClass(item.type)}">{item.type}</span>
                <span class="breakdown-count mono">{item.count} <span class="breakdown-pct">({item.pct}%)</span></span>
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
        <h3>Activity Feed</h3>
        <span class="badge badge-info">{feedEntries.length}</span>
      </div>
      {#if feedEntries.length > 0}
        <div class="feed-list">
          {#each feedEntries as entry}
            <div class="feed-item">
              <span class="feed-icon">{typeIcon(entry.operation || entry.Operation)}</span>
              <div class="feed-content">
                <div class="feed-title">
                  <span class="badge {typeBadgeClass(entry.operation || entry.Operation)}" style="font-size:10px">
                    {entry.operation || entry.Operation || '—'}
                  </span>
                  <span class="feed-entity mono">
                    {entry.entityType || entry.EntityType || ''}{entry.entityId || entry.EntityId ? ` #${truncate(String(entry.entityId || entry.EntityId), 12)}` : ''}
                  </span>
                </div>
                {#if entry.details || entry.Details || entry.narrative || entry.Narrative}
                  <p class="feed-narrative">{truncate(entry.details || entry.Details || entry.narrative || entry.Narrative || '', 120)}</p>
                {/if}
              </div>
              <span class="feed-time">{timeAgo(entry.timestamp || entry.Timestamp || entry.createdAt)}</span>
            </div>
          {/each}
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
    align-items: center;
    justify-content: space-between;
    margin-bottom: 16px;
  }
  .section-header h3 {
    font-size: 15px;
    font-weight: 600;
  }
  .refresh-info {
    font-size: 11px;
    color: var(--text-muted);
  }

  /* Heatmap */
  .heatmap-card {
    margin-bottom: 20px;
  }
  .heatmap-wrapper {
    overflow-x: auto;
  }
  .heatmap-months {
    display: grid;
    grid-template-columns: 32px repeat(52, 1fr);
    margin-bottom: 4px;
  }
  .month-label {
    font-size: 10px;
    color: var(--text-muted);
    text-transform: uppercase;
  }
  .heatmap-container {
    display: flex;
    gap: 4px;
  }
  .heatmap-days {
    display: flex;
    flex-direction: column;
    gap: 2px;
    width: 28px;
    flex-shrink: 0;
  }
  .heatmap-days span {
    height: 12px;
    font-size: 9px;
    color: var(--text-muted);
    line-height: 12px;
  }
  .heatmap-grid {
    display: grid;
    gap: 2px;
    flex: 1;
  }
  .heatmap-col {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .heatmap-cell {
    width: 12px;
    height: 12px;
    border-radius: 2px;
    transition: opacity 0.15s;
  }
  .heatmap-cell:hover {
    opacity: 0.8;
    outline: 1px solid var(--text-muted);
  }
  .level-0 { background: var(--bg-hover); }
  .level-1 { background: rgba(34, 197, 94, 0.25); }
  .level-2 { background: rgba(34, 197, 94, 0.45); }
  .level-3 { background: rgba(34, 197, 94, 0.7); }
  .level-4 { background: var(--success); }

  .heatmap-legend {
    display: flex;
    align-items: center;
    gap: 4px;
    justify-content: flex-end;
    margin-top: 8px;
  }
  .legend-label {
    font-size: 10px;
    color: var(--text-muted);
  }
  .heatmap-legend .heatmap-cell {
    width: 10px;
    height: 10px;
  }

  /* Content grid */
  .content-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 20px;
  }
  @media (max-width: 900px) { .content-grid { grid-template-columns: 1fr; } }

  /* Type Breakdown */
  .breakdown-list {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .breakdown-item {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .breakdown-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
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
    height: 6px;
    background: var(--bg-hover);
    border-radius: 3px;
    overflow: hidden;
  }
  .breakdown-bar-fill {
    height: 100%;
    background: var(--accent);
    border-radius: 3px;
    transition: width 0.3s ease;
    min-width: 2px;
  }

  /* Activity Feed */
  .feed-list {
    display: flex;
    flex-direction: column;
    gap: 2px;
    max-height: 500px;
    overflow-y: auto;
  }
  .feed-item {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    padding: 10px 8px;
    border-bottom: 1px solid var(--border);
    transition: background 0.15s;
  }
  .feed-item:hover {
    background: var(--bg-hover);
  }
  .feed-item:last-child { border-bottom: none; }
  .feed-icon {
    font-size: 16px;
    flex-shrink: 0;
    margin-top: 1px;
  }
  .feed-content {
    flex: 1;
    min-width: 0;
  }
  .feed-title {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
  }
  .feed-entity {
    font-size: 12px;
    color: var(--text-secondary);
  }
  .feed-narrative {
    font-size: 12px;
    color: var(--text-muted);
    margin-top: 4px;
    line-height: 1.4;
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
    padding: 20px 0;
  }

  /* Loading skeletons */
  .loading-state {
    display: flex;
    flex-direction: column;
    gap: 20px;
  }
  .skeleton-heatmap {
    height: 120px;
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    animation: pulse 1.5s ease-in-out infinite;
  }
  .skeleton-bars {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 20px;
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
  }
  .skeleton-bar {
    height: 24px;
    background: var(--bg-hover);
    border-radius: 4px;
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
