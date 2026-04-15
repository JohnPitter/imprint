<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api } from '../../lib/api';
  import { timeAgo, formatNumber } from '../../lib/format';

  let health: any = null;
  let sessions: any[] = [];
  let graph: any = null;
  let auditEntries: any[] = [];
  let memoriesCount = 0;
  let lessonsCount = 0;
  let crystalsCount = 0;
  let loading = true;
  let lastRefresh = '';
  let interval: ReturnType<typeof setInterval>;

  async function refresh() {
    try {
      const [h, s, g, a, m, l, c] = await Promise.all([
        api.health().catch(() => null),
        api.listSessions(5).catch(() => ({ sessions: [] })),
        api.graphStats().catch(() => null),
        api.listAudit(5, 0).catch(() => ({ entries: [] })),
        api.listMemories('', 0).catch(() => ({ memories: [], total: 0 })),
        api.listLessons(0).catch(() => ({ lessons: [], total: 0 })),
        api.listCrystals(0).catch(() => ({ crystals: [], total: 0 })),
      ]);
      health = h;
      sessions = (s as any).sessions || [];
      graph = g;
      auditEntries = (a as any).entries || (a as any).audit || [];
      memoriesCount = (m as any).total ?? ((m as any).memories?.length || 0);
      lessonsCount = (l as any).total ?? ((l as any).lessons?.length || 0);
      crystalsCount = (c as any).total ?? ((c as any).crystals?.length || 0);
      lastRefresh = new Date().toLocaleTimeString();
    } catch (e) {
      console.error('Dashboard refresh error:', e);
    }
    loading = false;
  }

  onMount(() => {
    refresh();
    interval = setInterval(refresh, 30000);
  });
  onDestroy(() => clearInterval(interval));

  function statusColor(status: string): string {
    if (!status) return '';
    const s = status.toLowerCase();
    if (s === 'healthy' || s === 'ok') return 'badge-success';
    if (s === 'degraded' || s === 'warning') return 'badge-warning';
    return 'badge-danger';
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
    if (typeof raw === 'string') return raw;
    if (typeof raw === 'number') {
      const h = Math.floor(raw / 3600);
      const m = Math.floor((raw % 3600) / 60);
      const s = Math.floor(raw % 60);
      if (h > 0) return `${h}h ${m}m ${s}s`;
      if (m > 0) return `${m}m ${s}s`;
      return `${s}s`;
    }
    return '—';
  }
</script>

{#if loading}
  <div class="loading-grid">
    {#each Array(6) as _}
      <div class="card skeleton-card"><div class="skeleton-value"></div><div class="skeleton-label"></div></div>
    {/each}
  </div>
{:else}
  <!-- Stats Grid -->
  <div class="stats-grid">
    <div class="card stat-card">
      <div class="stat-icon">📋</div>
      <div class="stat-value">{formatNumber(sessions.length)}</div>
      <div class="stat-label">Sessions</div>
    </div>
    <div class="card stat-card">
      <div class="stat-icon">🧠</div>
      <div class="stat-value">{formatNumber(memoriesCount)}</div>
      <div class="stat-label">Memories</div>
    </div>
    <div class="card stat-card">
      <div class="stat-icon">📖</div>
      <div class="stat-value">{formatNumber(lessonsCount)}</div>
      <div class="stat-label">Lessons</div>
    </div>
    <div class="card stat-card">
      <div class="stat-icon">💎</div>
      <div class="stat-value">{formatNumber(crystalsCount)}</div>
      <div class="stat-label">Crystals</div>
    </div>
    <div class="card stat-card">
      <div class="stat-icon">🔗</div>
      <div class="stat-value">{formatNumber(graph?.totalNodes || graph?.nodes || 0)}</div>
      <div class="stat-label">Graph Nodes</div>
    </div>
    <div class="card stat-card">
      <div class="stat-icon">↔️</div>
      <div class="stat-value">{formatNumber(graph?.totalEdges || graph?.edges || 0)}</div>
      <div class="stat-label">Graph Edges</div>
    </div>
  </div>

  <!-- System Health -->
  <div class="card health-card">
    <div class="section-header">
      <h3>System Health</h3>
      <span class="refresh-info mono">Auto-refresh 30s · Last: {lastRefresh}</span>
    </div>
    {#if health}
      <div class="health-grid">
        <div class="health-item">
          <span class="health-key">Status</span>
          <span class="badge {statusColor(health.status)}">{health.status || 'unknown'}</span>
        </div>
        <div class="health-item">
          <span class="health-key">Uptime</span>
          <span class="health-val mono">{formatUptime(health.uptime || health.uptimeSeconds)}</span>
        </div>
        <div class="health-item">
          <span class="health-key">Go Version</span>
          <span class="health-val mono">{health.goVersion || '—'}</span>
        </div>
        <div class="health-item">
          <span class="health-key">Goroutines</span>
          <span class="health-val mono">{health.goroutines ?? '—'}</span>
        </div>
        <div class="health-item">
          <span class="health-key">Alloc Memory</span>
          <span class="health-val mono">{health.memory?.allocMB?.toFixed(1) ?? '—'} MB</span>
        </div>
        <div class="health-item">
          <span class="health-key">Sys Memory</span>
          <span class="health-val mono">{health.memory?.sysMB?.toFixed(1) ?? '—'} MB</span>
        </div>
        <div class="health-item">
          <span class="health-key">GC Cycles</span>
          <span class="health-val mono">{health.memory?.numGC ?? '—'}</span>
        </div>
      </div>
    {:else}
      <p class="no-data">Unable to fetch health data</p>
    {/if}
  </div>

  <!-- Bottom row: Sessions + Audit side by side -->
  <div class="bottom-grid">
    <!-- Recent Sessions -->
    <div class="card">
      <div class="section-header">
        <h3>Recent Sessions</h3>
        <span class="badge badge-info">{sessions.length}</span>
      </div>
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
                <td style="color:var(--text-muted);font-size:12px">{timeAgo(s.StartedAt || s.startedAt || s.createdAt)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      {:else}
        <p class="no-data">No sessions yet</p>
      {/if}
    </div>

    <!-- Recent Audit Feed -->
    <div class="card">
      <div class="section-header">
        <h3>Recent Audit</h3>
        <span class="badge badge-purple">{auditEntries.length}</span>
      </div>
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
        <p class="no-data">No audit entries</p>
      {/if}
    </div>
  </div>
{/if}

<style>
  .stats-grid {
    display: grid;
    grid-template-columns: repeat(6, 1fr);
    gap: 16px;
    margin-bottom: 20px;
  }
  @media (max-width: 1200px) { .stats-grid { grid-template-columns: repeat(3, 1fr); } }
  @media (max-width: 640px) { .stats-grid { grid-template-columns: repeat(2, 1fr); } }

  .stat-card {
    text-align: center;
    padding: 24px 16px;
  }
  .stat-icon {
    font-size: 20px;
    margin-bottom: 8px;
    opacity: 0.7;
  }

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

  .health-card {
    margin-bottom: 20px;
  }
  .health-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
    gap: 12px;
  }
  .health-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 12px;
    background: var(--bg-secondary);
    border-radius: var(--radius);
  }
  .health-key {
    font-size: 12px;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }
  .health-val {
    font-size: 13px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .bottom-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 20px;
  }
  @media (max-width: 900px) { .bottom-grid { grid-template-columns: 1fr; } }

  .audit-feed {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .audit-item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 0;
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
    padding: 20px 0;
  }

  .loading-grid {
    display: grid;
    grid-template-columns: repeat(6, 1fr);
    gap: 16px;
  }
  @media (max-width: 1200px) { .loading-grid { grid-template-columns: repeat(3, 1fr); } }
  .skeleton-card {
    padding: 24px 16px;
    text-align: center;
  }
  .skeleton-value {
    width: 60px;
    height: 28px;
    background: var(--bg-hover);
    border-radius: 4px;
    margin: 0 auto 8px;
    animation: pulse 1.5s ease-in-out infinite;
  }
  .skeleton-label {
    width: 80px;
    height: 12px;
    background: var(--bg-hover);
    border-radius: 4px;
    margin: 0 auto;
    animation: pulse 1.5s ease-in-out infinite;
  }
  @keyframes pulse {
    0%, 100% { opacity: 0.4; }
    50% { opacity: 0.8; }
  }
</style>
