<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { flip } from 'svelte/animate';
  import { api } from '../../lib/api';
  import { createPoller } from '../../lib/poller';

  let pending: any[] = $state([]);
  let inProgress: any[] = $state([]);
  let done: any[] = $state([]);
  let frontier: any[] = $state([]);
  let loading = $state(true);
  let doneOffset = $state(0);
  // Visual indicator that the SSE stream is connected. Pulses on every push.
  let live = $state(false);
  let pulseAt = $state(0);
  const doneLimit = 30;
  // Pending/in_progress columns load everything; only "done" paginates because it grows unbounded.
  const activeLimit = 200;
  let stopPoll: (() => void) | undefined;
  let evtSource: EventSource | undefined;

  onMount(() => {
    load(true);
    // Two refresh paths run side-by-side:
    //   1. EventSource — server pushes "actions:changed" the instant a row
    //      moves between statuses; we re-fetch immediately for kanban
    //      updates that feel real-time.
    //   2. Poll fallback every 5s — covers proxies that strip SSE, lost
    //      reconnects, and the very first paint before the stream opens.
    connectStream();
    stopPoll = createPoller(() => load(false), 5000);
  });

  onDestroy(() => {
    stopPoll?.();
    evtSource?.close();
  });

  function connectStream() {
    try {
      evtSource = new EventSource('/imprint/actions/stream');
      evtSource.onopen = () => { live = true; };
      evtSource.onmessage = () => {
        pulseAt = Date.now();
        load(false);
      };
      evtSource.onerror = () => {
        live = false;
        // EventSource auto-reconnects with backoff; no manual handling needed.
      };
    } catch {
      live = false;
    }
  }

  // initial=true shows the skeleton; subsequent polls update silently in place.
  async function load(initial: boolean) {
    if (initial) loading = true;
    try {
      const [p, ip, d, f] = await Promise.all([
        api.listActions('pending', activeLimit, 0),
        api.listActions('in_progress', activeLimit, 0),
        api.listActions('done', doneLimit, doneOffset),
        api.frontier(),
      ]);
      pending = p.actions || [];
      inProgress = ip.actions || [];
      done = d.actions || [];
      frontier = f.actions || [];
    } catch(e) { console.error(e); }
    if (initial) loading = false;
  }

  function prev() { if (doneOffset >= doneLimit) { doneOffset -= doneLimit; load(true); } }
  function next() { if (done.length >= doneLimit) { doneOffset += doneLimit; load(true); } }

  let currentPage = $derived(Math.floor(doneOffset / doneLimit) + 1);
  let totalPages = $derived(done.length < doneLimit ? currentPage : currentPage + 1);
  let anyActions = $derived(pending.length + inProgress.length + done.length > 0);

  function priorityBadge(p: number): string {
    if (p >= 8) return 'act-priority-high';
    if (p >= 5) return 'act-priority-med';
    return 'act-priority-low';
  }

  // Build the session label shown on each card. Prefer the project name
  // (human-readable) and fall back to a short slice of the session id when
  // project is missing. Returns null for legacy actions with no session
  // attached so the badge is hidden entirely instead of rendering empty.
  function sessionLabel(a: any): string | null {
    if (a.project) {
      const last = String(a.project).split('/').filter(Boolean).pop() || a.project;
      const sid = a.sessionId ? String(a.sessionId).slice(0, 6) : '';
      return sid ? `${last} · ${sid}` : last;
    }
    if (a.sessionId) return String(a.sessionId).slice(0, 8);
    return null;
  }
</script>

{#if loading}
  <div class="act-loading">
    <span class="act-loading-label">LOADING ACTIONS</span>
  </div>
{:else}
  <!-- Frontier Section -->
  {#if frontier.length > 0}
    <div class="act-frontier">
      <div class="act-frontier-header">
        <span class="act-frontier-label">NEXT UP</span>
        <span class="act-frontier-count">{frontier.length}</span>
      </div>
      <div class="act-frontier-list">
        {#each frontier as a}
          <div class="act-frontier-item">
            <span class="act-priority {priorityBadge(a.priority)}">P{a.priority}</span>
            <span class="act-frontier-title">{a.title}</span>
            {#if a.description}
              <span class="act-frontier-desc">{'\u2014'} {a.description}</span>
            {/if}
          </div>
        {/each}
      </div>
    </div>
  {/if}

  <!-- Kanban -->
  {#if !anyActions}
    <div class="empty-state">
      <div class="act-empty-icon">{'\u25A0'}</div>
      <p style="font-family:var(--font-ui);font-size:13px">No actions yet</p>
    </div>
  {:else}
    <div class="act-live-row">
      <span class="act-live-dot {live ? 'live-on' : 'live-off'}" class:pulse={pulseAt > 0 && Date.now() - pulseAt < 800}></span>
      <span class="act-live-label">{live ? 'LIVE' : 'POLLING'}</span>
    </div>

    <div class="act-kanban">
      <!-- Pending Column -->
      <div class="act-column">
        <div class="act-col-header">
          <div class="act-col-title-row">
            <span class="act-col-label">PENDING</span>
            <span class="act-col-count">{pending.length}</span>
          </div>
          <div class="act-col-underline"></div>
        </div>
        <div class="act-col-body">
          {#each pending as a (a.id)}
            <div class="act-card" animate:flip={{ duration: 250 }}>
              <div class="act-card-top">
                <span class="act-priority {priorityBadge(a.priority)}">P{a.priority}</span>
                <strong class="act-card-title">{a.title}</strong>
              </div>
              {#if a.description}
                <p class="act-card-desc">{a.description}</p>
              {/if}
              {#if sessionLabel(a)}
                <div class="act-card-session" title={a.sessionId || ''}>{sessionLabel(a)}</div>
              {/if}
            </div>
          {/each}
          {#if pending.length === 0}
            <div class="act-col-empty">{'\u2014'}</div>
          {/if}
        </div>
      </div>

      <!-- Divider -->
      <div class="act-divider"></div>

      <!-- In Progress Column -->
      <div class="act-column">
        <div class="act-col-header">
          <div class="act-col-title-row">
            <span class="act-col-label">IN PROGRESS</span>
            <span class="act-col-count">{inProgress.length}</span>
          </div>
          <div class="act-col-underline"></div>
        </div>
        <div class="act-col-body">
          {#each inProgress as a (a.id)}
            <div class="act-card" animate:flip={{ duration: 250 }}>
              <div class="act-card-top">
                <span class="act-priority {priorityBadge(a.priority)}">P{a.priority}</span>
                <strong class="act-card-title">{a.title}</strong>
              </div>
              {#if a.description}
                <p class="act-card-desc">{a.description}</p>
              {/if}
              {#if sessionLabel(a)}
                <div class="act-card-session" title={a.sessionId || ''}>{sessionLabel(a)}</div>
              {/if}
            </div>
          {/each}
          {#if inProgress.length === 0}
            <div class="act-col-empty">{'\u2014'}</div>
          {/if}
        </div>
      </div>

      <!-- Divider -->
      <div class="act-divider"></div>

      <!-- Done Column -->
      <div class="act-column">
        <div class="act-col-header">
          <div class="act-col-title-row">
            <span class="act-col-label">DONE</span>
            <span class="act-col-count">{done.length}</span>
          </div>
          <div class="act-col-underline"></div>
        </div>
        <div class="act-col-body">
          {#each done as a (a.id)}
            <div class="act-card act-card-done" animate:flip={{ duration: 250 }}>
              <div class="act-card-top">
                <span class="act-done-mark" aria-hidden="true">{'✓'}</span>
                <strong class="act-card-title">{a.title}</strong>
              </div>
              {#if a.description}
                <p class="act-card-desc">{a.description}</p>
              {/if}
              {#if sessionLabel(a)}
                <div class="act-card-session" title={a.sessionId || ''}>{sessionLabel(a)}</div>
              {/if}
            </div>
          {/each}
          {#if done.length === 0}
            <div class="act-col-empty">{'\u2014'}</div>
          {/if}
        </div>
      </div>
    </div>

    <div class="pagination">
      <button class="pagination-btn" onclick={prev} disabled={doneOffset === 0}>{'\u2190'} PREV</button>
      <span class="pagination-info">DONE PAGE {currentPage} OF {totalPages}</span>
      <button class="pagination-btn" onclick={next} disabled={done.length < doneLimit}>NEXT {'\u2192'}</button>
    </div>
  {/if}
{/if}

<style>
  /* Loading */
  .act-loading {
    padding: 40px 24px;
  }
  .act-loading-label {
    font-family: var(--font-ui);
    font-size: 10px;
    font-weight: 600;
    color: var(--text-muted);
    letter-spacing: 0.12em;
    animation: pulse 1.4s infinite;
  }
  @keyframes pulse { 0%, 100% { opacity: 0.3; } 50% { opacity: 1; } }

  .act-empty-icon {
    font-size: 28px;
    color: var(--accent);
    opacity: 0.2;
    margin-bottom: 16px;
  }

  /* Frontier */
  .act-frontier {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-top: 2px solid var(--accent);
    padding: 24px;
    margin-bottom: 28px;
    transition: box-shadow 0.3s var(--ease);
  }
  .act-frontier:hover {
    box-shadow: var(--shadow-hover);
  }
  .act-frontier-header {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 16px;
  }
  .act-frontier-label {
    font-family: var(--font-ui);
    font-size: 10px;
    font-weight: 700;
    color: var(--accent);
    text-transform: uppercase;
    letter-spacing: 0.12em;
  }
  .act-frontier-count {
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--text-dim);
  }
  .act-frontier-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .act-frontier-item {
    display: flex;
    align-items: baseline;
    gap: 10px;
    padding: 8px 0;
    border-bottom: 1px solid var(--border);
  }
  .act-frontier-item:last-child {
    border-bottom: none;
  }
  .act-frontier-title {
    font-family: var(--font-ui);
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
  }
  .act-frontier-desc {
    font-family: var(--font-ui);
    font-size: 12px;
    color: var(--text-muted);
  }

  /* Priority badges */
  .act-priority {
    display: inline-flex;
    align-items: center;
    padding: 2px 8px;
    font-family: var(--font-mono);
    font-size: 11px;
    font-weight: 600;
    border: 1px solid transparent;
    flex-shrink: 0;
  }
  .act-priority-high {
    color: var(--accent);
    border-color: rgba(200, 147, 58, 0.3);
    background: rgba(200, 147, 58, 0.08);
  }
  .act-priority-med {
    color: var(--text-dim);
    border-color: var(--border);
    background: transparent;
  }
  .act-priority-low {
    color: var(--text-muted);
    border-color: var(--border);
    background: transparent;
  }

  /* Kanban */
  .act-kanban {
    display: grid;
    grid-template-columns: 1fr auto 1fr auto 1fr;
    gap: 0;
    min-height: 300px;
  }
  .act-divider {
    width: 1px;
    background: var(--border);
  }

  .act-column {
    min-height: 200px;
    display: flex;
    flex-direction: column;
  }
  .act-col-header {
    padding: 0 24px 16px;
  }
  .act-col-title-row {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 8px;
  }
  .act-col-label {
    font-family: var(--font-ui);
    font-size: 10px;
    font-weight: 700;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.1em;
  }
  .act-col-count {
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--text-dim);
  }
  .act-col-underline {
    width: 24px;
    height: 2px;
    background: var(--accent);
  }

  .act-col-body {
    padding: 0 24px;
    display: flex;
    flex-direction: column;
    gap: 6px;
    flex: 1;
  }
  .act-col-empty {
    color: var(--text-muted);
    font-size: 13px;
    padding: 12px 0;
    opacity: 0.4;
  }

  /* Action cards */
  .act-card {
    padding: 16px 18px;
    border: 1px solid var(--border);
    background: var(--bg-card);
    transition: all 0.2s var(--ease);
  }
  .act-card:hover {
    border-color: var(--accent);
    box-shadow: var(--shadow-hover);
  }
  .act-card-done {
    opacity: 0.78;
  }
  .act-card-done:hover {
    opacity: 1;
  }
  .act-done-mark {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--accent);
    flex-shrink: 0;
  }
  .act-card-top {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .act-card-title {
    font-family: var(--font-ui);
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
  }
  .act-card-desc {
    font-family: var(--font-ui);
    font-size: 13px;
    color: var(--text-muted);
    margin-top: 6px;
    line-height: 1.5;
  }

  /* Session badge: a small monospace tag at the bottom of each card so
     the user can tell at a glance which Claude Code session produced
     this action. Truncates with ellipsis on narrow columns. */
  .act-card-session {
    margin-top: 8px;
    font-family: var(--font-mono);
    font-size: 10px;
    color: var(--text-dim);
    letter-spacing: 0.04em;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* Live indicator: a small dot + label at the top of the kanban that
     turns the page from "polled" to "live". Pulses briefly each time
     an SSE push arrives so the user has a visible cue that the data
     just refreshed. */
  .act-live-row {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 14px;
  }
  .act-live-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    transition: box-shadow 0.3s ease, background 0.3s ease;
  }
  .live-on { background: #34d399; box-shadow: 0 0 6px rgba(52,211,153,0.6); }
  .live-off { background: var(--text-muted); }
  .pulse {
    animation: act-pulse 0.7s ease-out;
  }
  @keyframes act-pulse {
    0% { transform: scale(1); }
    40% { transform: scale(1.6); box-shadow: 0 0 12px rgba(52,211,153,0.9); }
    100% { transform: scale(1); }
  }
  .act-live-label {
    font-family: var(--font-ui);
    font-size: 10px;
    font-weight: 700;
    color: var(--text-muted);
    letter-spacing: 0.12em;
  }
</style>
