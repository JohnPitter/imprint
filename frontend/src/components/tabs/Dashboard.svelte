<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api } from '../../lib/api';
  import { createPoller } from '../../lib/poller';
  import { timeAgo, formatNumber } from '../../lib/format';

  let health: any = null;
  let sessions: any[] = [];
  let sessionsTotal = 0;
  let graph: any = null;
  let auditEntries: any[] = [];
  let memoriesCount = 0;
  let lessonsCount = 0;
  let loading = true;
  let lastRefresh = '';
  let stopPoll: (() => void) | undefined;

  async function refresh() {
    try {
      const [h, s, g, a, m, l] = await Promise.all([
        api.health().catch(() => null),
        // Pull a generous page so the recent-list and total agree, then trim
        // for display below.
        api.listSessions(50, 0).catch(() => ({ sessions: [], total: 0 })),
        api.graphStats().catch(() => null),
        api.listAudit(5, 0).catch(() => ({ entries: [] })),
        api.listMemories('', 1, 0).catch(() => ({ memories: [], total: 0 })),
        api.listLessons(1, 0).catch(() => ({ lessons: [], total: 0 })),
      ]);
      health = h;
      sessions = ((s as any).sessions || []).slice(0, 5);
      sessionsTotal = (s as any).total ?? ((s as any).sessions?.length || 0);
      graph = g;
      auditEntries = (a as any).entries || (a as any).audit || [];
      memoriesCount = (m as any).total ?? ((m as any).memories?.length || 0);
      lessonsCount = (l as any).total ?? ((l as any).lessons?.length || 0);
    } catch (e) {
      console.error('Dashboard refresh error:', e);
    }
    lastRefresh = new Date().toLocaleTimeString();
    loading = false;
  }

  onMount(() => {
    refresh();
    stopPoll = createPoller(refresh, 30000);
  });
  onDestroy(() => stopPoll?.());

  function healthStatusDot(status: string): string {
    if (!status) return 'idle';
    const s = status.toLowerCase();
    if (s === 'healthy' || s === 'ok') return 'active';
    if (s === 'degraded' || s === 'warning') return 'warning';
    return 'error';
  }

  function sessionStatusBadge(status: string): string {
    if (!status) return '';
    const s = status.toLowerCase();
    if (s === 'completed' || s === 'ended') return 'badge-success';
    if (s === 'active') return 'badge-accent';
    if (s === 'failed' || s === 'error') return 'badge-danger';
    return 'badge-info';
  }

  function auditOpBadge(op: string): string {
    if (!op) return 'badge-info';
    const o = op.toLowerCase();
    if (o.includes('create') || o.includes('add') || o.includes('start')) return 'badge-success';
    if (o.includes('delete') || o.includes('remove') || o.includes('forget')) return 'badge-danger';
    if (o.includes('update') || o.includes('evolve') || o.includes('merge')) return 'badge-warning';
    return 'badge-info';
  }

  function formatUptime(raw: any): string {
    if (typeof raw === 'number') {
      const h = Math.floor(raw / 3600);
      const m = Math.floor((raw % 3600) / 60);
      const s = Math.floor(raw % 60);
      if (h > 0) return `${h}h ${m}m ${s}s`;
      if (m > 0) return `${m}m ${s}s`;
      return `${s}s`;
    }
    if (typeof raw === 'string') {
      // Parse Go duration format: "1h34m25.2628827s"
      const match = raw.match(/(?:(\d+)h)?(?:(\d+)m)?(?:(\d+)(?:\.\d+)?s)?/);
      if (match) {
        const h = parseInt(match[1] || '0');
        const m = parseInt(match[2] || '0');
        const s = parseInt(match[3] || '0');
        if (h > 0) return `${h}h ${m}m ${s}s`;
        if (m > 0) return `${m}m ${s}s`;
        return `${s}s`;
      }
      return raw;
    }
    return '—';
  }
</script>

{#if loading}
  <div class="loading-grid">
    {#each Array(6) as _}
      <div class="stat-card-shell">
        <div class="skeleton-value"></div>
        <div class="skeleton-label"></div>
      </div>
    {/each}
  </div>
{:else}
  <!-- Last updated -->
  <div class="refresh-row">
    <span class="refresh-text mono">Last updated: {lastRefresh}</span>
  </div>

  <!-- Stats Grid -->
  <div class="stats-grid">
    <div class="stat-card">
      <div class="stat-value">{formatNumber(sessionsTotal)}</div>
      <div class="stat-label">Sessions</div>
    </div>
    <div class="stat-card">
      <div class="stat-value">{formatNumber(memoriesCount)}</div>
      <div class="stat-label">Memories</div>
    </div>
    <div class="stat-card">
      <div class="stat-value">{formatNumber(lessonsCount)}</div>
      <div class="stat-label">Lessons</div>
    </div>
    <div class="stat-card">
      <div class="stat-value">{formatNumber(graph?.totalNodes || graph?.nodes || 0)}</div>
      <div class="stat-label">Graph Nodes</div>
    </div>
    <div class="stat-card">
      <div class="stat-value">{formatNumber(graph?.totalEdges || graph?.edges || 0)}</div>
      <div class="stat-label">Graph Edges</div>
    </div>
  </div>

  <!-- Gold separator -->
  <div class="gold-line"></div>

  <!-- System Health -->
  <div class="section-block">
    <h3 class="section-heading">System Health</h3>
    {#if health}
      <div class="health-bar">
        <div class="health-pair">
          <span class="health-key">Status</span>
          <span class="health-val">
            <span class="status-dot" data-status={healthStatusDot(health.status)}></span>
            {health.status || 'unknown'}
          </span>
        </div>
        <div class="health-divider"></div>
        <div class="health-pair">
          <span class="health-key">Uptime</span>
          <span class="health-val mono">{formatUptime(health.uptime || health.uptimeSeconds)}</span>
        </div>
        <div class="health-divider"></div>
        <div class="health-pair">
          <span class="health-key">Go Version</span>
          <span class="health-val mono">{health.goVersion || '—'}</span>
        </div>
        <div class="health-divider"></div>
        <div class="health-pair">
          <span class="health-key">Goroutines</span>
          <span class="health-val mono">{health.goroutines ?? '—'}</span>
        </div>
        <div class="health-divider"></div>
        <div class="health-pair">
          <span class="health-key">Alloc</span>
          <span class="health-val mono">{health.memory?.allocMB?.toFixed(1) ?? '—'} MB</span>
        </div>
        <div class="health-divider"></div>
        <div class="health-pair">
          <span class="health-key">Sys</span>
          <span class="health-val mono">{health.memory?.sysMB?.toFixed(1) ?? '—'} MB</span>
        </div>
        <div class="health-divider"></div>
        <div class="health-pair">
          <span class="health-key">GC Cycles</span>
          <span class="health-val mono">{health.memory?.numGC ?? '—'}</span>
        </div>
      </div>
    {:else}
      <p class="no-data">Unable to fetch health data</p>
    {/if}
  </div>

  <!-- Gold separator -->
  <div class="gold-line"></div>

  <!-- Bottom row: Sessions + Audit -->
  <div class="bottom-grid">
    <!-- Recent Sessions -->
    <div class="section-block">
      <h3 class="section-heading">Recent Sessions</h3>
      {#if sessions.length > 0}
        <table>
          <thead>
            <tr><th>Project</th><th>Status</th><th>Obs</th><th>Started</th></tr>
          </thead>
          <tbody>
            {#each sessions as s}
              <tr>
                <td class="mono">{s.Project || s.project || '—'}</td>
                <td>
                  <span class="badge {sessionStatusBadge(s.Status || s.status)}">
                    {s.Status || s.status || '—'}
                  </span>
                </td>
                <td class="mono">{s.ObservationCount || s.observationCount || 0}</td>
                <td class="cell-muted">{timeAgo(s.StartedAt || s.startedAt || s.createdAt)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      {:else}
        <div class="empty-state">
          <div class="icon">—</div>
          <p>No sessions yet</p>
        </div>
      {/if}
    </div>

    <!-- Recent Audit Feed -->
    <div class="section-block">
      <h3 class="section-heading">Recent Audit</h3>
      {#if auditEntries.length > 0}
        <div class="audit-feed">
          {#each auditEntries as entry}
            <div class="audit-item">
              <span class="badge {auditOpBadge(entry.operation || entry.Operation)}">
                {entry.operation || entry.Operation || '—'}
              </span>
              <span class="audit-entity mono">
                {entry.entityType || entry.EntityType || ''}{entry.entityId || entry.EntityId ? ` #${entry.entityId || entry.EntityId}` : ''}
              </span>
              <span class="audit-time">{timeAgo(entry.timestamp || entry.Timestamp || entry.createdAt)}</span>
            </div>
          {/each}
        </div>
      {:else}
        <div class="empty-state">
          <div class="icon">—</div>
          <p>No audit entries</p>
        </div>
      {/if}
    </div>
  </div>
{/if}

<style>
  /* Refresh row */
  .refresh-row {
    display: flex;
    justify-content: flex-end;
    margin-bottom: 24px;
  }
  .refresh-text {
    font-size: 11px;
    color: var(--text-muted);
    letter-spacing: 0.04em;
  }

  /* Stats Grid */
  .stats-grid {
    display: grid;
    grid-template-columns: repeat(5, 1fr);
    gap: 16px;
    margin-bottom: 32px;
  }
  @media (max-width: 1200px) { .stats-grid { grid-template-columns: repeat(3, 1fr); } }
  @media (max-width: 640px) { .stats-grid { grid-template-columns: repeat(2, 1fr); } }

  .stat-card {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-top: 2px solid var(--accent);
    padding: 24px 16px;
    text-align: center;
    transition: border-color 0.3s var(--ease), box-shadow 0.3s var(--ease);
  }
  .stat-card:hover {
    border-color: var(--accent);
    box-shadow: var(--shadow-hover);
  }

  /* Section blocks */
  .section-block {
    margin-bottom: 8px;
  }
  .section-heading {
    font-family: var(--font-display);
    font-size: 18px;
    font-weight: 600;
    letter-spacing: -0.03em;
    margin-bottom: 16px;
    color: var(--text-primary);
  }

  /* Gold line */
  .gold-line {
    width: 40px;
    height: 2px;
    background: var(--accent);
    margin: 32px 0;
  }

  /* Health bar */
  .health-bar {
    display: flex;
    align-items: center;
    gap: 0;
    padding: 16px 20px;
    background: var(--bg-card);
    border: 1px solid var(--border);
    overflow-x: auto;
  }
  .health-pair {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 0 20px;
    white-space: nowrap;
  }
  .health-pair:first-child { padding-left: 0; }
  .health-pair:last-child { padding-right: 0; }
  .health-key {
    font-size: 10px;
    font-family: var(--font-ui);
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.1em;
    font-weight: 600;
  }
  .health-val {
    font-size: 13px;
    font-weight: 600;
    color: var(--text-primary);
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .health-divider {
    width: 1px;
    height: 32px;
    background: var(--border);
    flex-shrink: 0;
  }

  /* Bottom grid */
  .bottom-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 32px;
  }
  @media (max-width: 900px) { .bottom-grid { grid-template-columns: 1fr; } }

  .cell-muted {
    color: var(--text-muted);
    font-size: 12px;
  }

  /* Audit feed */
  .audit-feed {
    display: flex;
    flex-direction: column;
  }
  .audit-item {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 10px 0;
    border-bottom: 1px solid var(--border);
    font-size: 13px;
  }
  .audit-item:last-child { border-bottom: none; }
  .audit-entity {
    flex: 1;
    color: var(--text-secondary);
    font-size: 12px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .audit-time {
    font-size: 11px;
    color: var(--text-muted);
    white-space: nowrap;
  }

  .no-data {
    color: var(--text-muted);
    font-size: 13px;
    text-align: center;
    padding: 24px 0;
  }

  /* Loading skeleton */
  .loading-grid {
    display: grid;
    grid-template-columns: repeat(5, 1fr);
    gap: 16px;
  }
  @media (max-width: 1200px) { .loading-grid { grid-template-columns: repeat(3, 1fr); } }

  .stat-card-shell {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-top: 2px solid var(--border-hover);
    padding: 24px 16px;
    text-align: center;
  }
  .skeleton-value {
    width: 60px;
    height: 32px;
    background: var(--bg-hover);
    margin: 0 auto 8px;
    animation: pulse 1.5s ease-in-out infinite;
  }
  .skeleton-label {
    width: 80px;
    height: 10px;
    background: var(--bg-hover);
    margin: 0 auto;
    animation: pulse 1.5s ease-in-out infinite;
  }
  @keyframes pulse {
    0%, 100% { opacity: 0.3; }
    50% { opacity: 0.7; }
  }
</style>
