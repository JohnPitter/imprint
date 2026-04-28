<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api } from '../../lib/api';
  import { createPoller } from '../../lib/poller';

  let lessons: any[] = $state([]);
  let insights: any[] = $state([]);
  let lessonsTotal = $state(0);
  let insightsTotal = $state(0);
  let searchQuery = $state('');
  let searching = $state(false);
  let loading = $state(true);
  let lessonsOffset = $state(0);
  let insightsOffset = $state(0);
  const limit = 30;
  let stopPoll: (() => void) | undefined;

  onMount(() => {
    loadAll(true);
    stopPoll = createPoller(() => loadAll(false), 15000);
  });

  onDestroy(() => {
    stopPoll?.();
  });

  async function loadAll(initial: boolean) {
    if (initial) loading = true;
    try {
      // While the user is viewing search results we don't clobber the lessons column,
      // but we still keep the insights column live.
      const tasks: Promise<any>[] = [api.listInsights(limit, insightsOffset)];
      if (!searching) tasks.unshift(api.listLessons(limit, lessonsOffset));
      const results = await Promise.all(tasks);
      if (!searching) {
        const l = results[0];
        lessons = l.lessons || [];
        lessonsTotal = l.total ?? lessons.length;
      }
      const i = results[searching ? 0 : 1];
      insights = i.insights || [];
      insightsTotal = i.total ?? insights.length;
    } catch(e) { console.error(e); }
    if (initial) loading = false;
  }

  async function loadLessons() {
    try {
      const r = await api.listLessons(limit, lessonsOffset);
      lessons = r.lessons || [];
      lessonsTotal = r.total ?? lessons.length;
    } catch(e) { console.error(e); }
  }
  async function loadInsights() {
    try {
      const r = await api.listInsights(limit, insightsOffset);
      insights = r.insights || [];
      insightsTotal = r.total ?? insights.length;
    } catch(e) { console.error(e); }
  }

  function lessonsPrev() { if (lessonsOffset >= limit) { lessonsOffset -= limit; loadLessons(); } }
  function lessonsNext() { if (lessonsOffset + limit < lessonsTotal) { lessonsOffset += limit; loadLessons(); } }
  function insightsPrev() { if (insightsOffset >= limit) { insightsOffset -= limit; loadInsights(); } }
  function insightsNext() { if (insightsOffset + limit < insightsTotal) { insightsOffset += limit; loadInsights(); } }

  let lessonsPage = $derived(Math.floor(lessonsOffset / limit) + 1);
  let lessonsTotalPages = $derived(Math.max(1, Math.ceil(lessonsTotal / limit)));
  let insightsPage = $derived(Math.floor(insightsOffset / limit) + 1);
  let insightsTotalPages = $derived(Math.max(1, Math.ceil(insightsTotal / limit)));

  // Track which insights are expanded. Set rather than per-insight `expanded`
  // flag because the API returns plain rows we don't want to mutate.
  let expandedInsights: Set<string> = $state(new Set());

  function toggleInsight(id: string) {
    if (expandedInsights.has(id)) expandedInsights.delete(id);
    else expandedInsights.add(id);
    expandedInsights = new Set(expandedInsights);
  }

  function parseList(v: any): string[] {
    if (!v) return [];
    if (Array.isArray(v)) return v;
    if (typeof v === 'string') {
      try { const p = JSON.parse(v); return Array.isArray(p) ? p : []; } catch { return []; }
    }
    return [];
  }

  function fmtTime(ts: string | undefined | null): string {
    if (!ts) return '';
    try { return new Date(ts).toLocaleString(); } catch { return ts; }
  }

  async function doSearch() {
    const q = searchQuery.trim();
    if (!q) {
      // Empty submit clears the search and goes back to the live list.
      searching = false;
      lessonsOffset = 0;
      loadLessons();
      return;
    }
    searching = true;
    lessonsOffset = 0;
    try {
      const r = await api.searchLessons(q);
      lessons = r.lessons || [];
      lessonsTotal = lessons.length;
    } catch(e) { console.error(e); }
  }

  function parseTags(tags: any): string[] {
    if (!tags) return [];
    if (typeof tags === 'string') {
      try { return JSON.parse(tags); } catch { return []; }
    }
    return Array.isArray(tags) ? tags : [];
  }

  // Lessons being dismissed: shown as "(dismissing…)" until the response
  // returns and the next poll removes them. Set, not array, for O(1) lookup.
  let dismissingIds: Set<string> = $state(new Set());

  async function dismissLesson(id: string, e: MouseEvent) {
    e.stopPropagation();
    if (dismissingIds.has(id)) return;
    dismissingIds.add(id);
    dismissingIds = new Set(dismissingIds);
    try {
      await api.dismissLesson(id);
      // Optimistic: drop from current page so it disappears immediately,
      // poll will reconcile counts.
      lessons = lessons.filter((l: any) => l.id !== id);
      lessonsTotal = Math.max(0, lessonsTotal - 1);
    } catch (e) {
      console.error('dismiss lesson failed:', e);
    } finally {
      dismissingIds.delete(id);
      dismissingIds = new Set(dismissingIds);
    }
  }
</script>

<div class="lessons-container">
  <!-- Search -->
  <div class="search-row">
    <form onsubmit={(e) => { e.preventDefault(); doSearch(); }} class="search-form">
      <input
        class="input search-input"
        bind:value={searchQuery}
        placeholder="Search lessons..."
      />
    </form>
  </div>

  {#if loading}
    <div class="loading-blocks">
      {#each Array(4) as _}
        <div class="skeleton-block"></div>
      {/each}
    </div>
  {:else}
    <div class="split-layout">
      <!-- Lessons column -->
      <div class="column">
        <div class="column-header">
          <div class="section-label-row">
            <div class="gold-line"></div>
            <span class="section-label">LESSONS</span>
            <span class="section-count">{searching ? lessons.length : lessonsTotal}</span>
          </div>
          <div class="pagination compact">
            <button class="pagination-btn" onclick={lessonsPrev} disabled={searching || lessonsOffset === 0}>{'\u2190'}</button>
            <span class="pagination-info">{lessonsPage}/{lessonsTotalPages}</span>
            <button class="pagination-btn" onclick={lessonsNext} disabled={searching || lessonsOffset + limit >= lessonsTotal}>{'\u2192'}</button>
          </div>
        </div>

        <div class="column-scroll">
          {#if lessons.length === 0}
            <div class="empty-state" style="padding:32px"><p>No lessons yet</p></div>
          {:else}
            <div class="lessons-list">
              {#each lessons as l}
                <div class="lesson-card" class:lesson-card-dismissing={dismissingIds.has(l.id)}>
                  <p class="lesson-content">{l.content}</p>
                  <div class="lesson-meta">
                    <div class="gauge-row">
                      <div class="gauge"><div class="gauge-fill" style="width:{(l.confidence || 0) * 100}%"></div></div>
                      <span class="mono gauge-label">{Math.round((l.confidence || 0) * 100)}%</span>
                    </div>
                    <span class="mono reinforcement-count">{l.reinforcements || 0}x</span>
                    <button
                      class="lesson-dismiss"
                      onclick={(e) => dismissLesson(l.id, e)}
                      disabled={dismissingIds.has(l.id)}
                      title="Dismiss this lesson (soft delete)"
                    >{dismissingIds.has(l.id) ? '…' : 'dismiss'}</button>
                  </div>
                  {#if parseTags(l.tags).length > 0}
                    <div class="lesson-tags">
                      {#each parseTags(l.tags) as t}<span class="tag-badge">{t}</span>{/each}
                    </div>
                  {/if}
                </div>
              {/each}
            </div>
          {/if}
        </div>
      </div>

      <!-- Insights column -->
      <div class="column">
        <div class="column-header">
          <div class="section-label-row">
            <div class="gold-line"></div>
            <span class="section-label">INSIGHTS</span>
            <span class="section-count">{insightsTotal}</span>
          </div>
          <div class="pagination compact">
            <button class="pagination-btn" onclick={insightsPrev} disabled={insightsOffset === 0}>{'\u2190'}</button>
            <span class="pagination-info">{insightsPage}/{insightsTotalPages}</span>
            <button class="pagination-btn" onclick={insightsNext} disabled={insightsOffset + limit >= insightsTotal}>{'\u2192'}</button>
          </div>
        </div>

        <div class="column-scroll">
          {#if insights.length === 0}
            <div class="empty-state" style="padding:32px"><p>No insights yet</p></div>
          {:else}
            <div class="insights-list">
              {#each insights as i}
                {@const expanded = expandedInsights.has(i.id)}
                {@const concepts = parseList(i.sourceConceptCluster)}
                {@const memIds = parseList(i.sourceMemoryIds)}
                {@const lessonIds = parseList(i.sourceLessonIds)}
                {@const tags = parseList(i.tags)}
                <button class="insight-card" class:insight-card-expanded={expanded} onclick={() => toggleInsight(i.id)}>
                  <div class="insight-card-head">
                    <h4 class="insight-title">{i.title}</h4>
                    <span class="insight-toggle mono">{expanded ? '−' : '+'}</span>
                  </div>
                  {#if !expanded}
                    <p class="insight-content insight-content-collapsed">{i.content}</p>
                  {:else}
                    <p class="insight-content">{i.content}</p>
                    <div class="insight-meta">
                      <div class="insight-meta-row">
                        <span class="insight-meta-key">Confidence</span>
                        <div class="gauge"><div class="gauge-fill" style="width:{(i.confidence || 0) * 100}%"></div></div>
                        <span class="mono insight-meta-val">{Math.round((i.confidence || 0) * 100)}%</span>
                      </div>
                      <div class="insight-meta-row">
                        <span class="insight-meta-key">Reinforcements</span>
                        <span class="mono insight-meta-val">{i.reinforcements || 0}x</span>
                      </div>
                      {#if i.createdAt}
                        <div class="insight-meta-row">
                          <span class="insight-meta-key">Created</span>
                          <span class="insight-meta-val">{fmtTime(i.createdAt)}</span>
                        </div>
                      {/if}
                      {#if i.lastReinforcedAt}
                        <div class="insight-meta-row">
                          <span class="insight-meta-key">Last reinforced</span>
                          <span class="insight-meta-val">{fmtTime(i.lastReinforcedAt)}</span>
                        </div>
                      {/if}
                    </div>
                    {#if concepts.length > 0}
                      <div class="insight-tags">
                        <span class="insight-tags-label">CONCEPTS</span>
                        {#each concepts as c}<span class="tag-badge">{c}</span>{/each}
                      </div>
                    {/if}
                    {#if tags.length > 0}
                      <div class="insight-tags">
                        <span class="insight-tags-label">TAGS</span>
                        {#each tags as t}<span class="tag-badge">{t}</span>{/each}
                      </div>
                    {/if}
                    {#if memIds.length > 0 || lessonIds.length > 0}
                      <div class="insight-sources">
                        {#if memIds.length > 0}
                          <span class="insight-sources-label">{memIds.length} source memor{memIds.length === 1 ? 'y' : 'ies'}</span>
                        {/if}
                        {#if lessonIds.length > 0}
                          <span class="insight-sources-label">{lessonIds.length} source lesson{lessonIds.length === 1 ? '' : 's'}</span>
                        {/if}
                      </div>
                    {/if}
                  {/if}
                </button>
              {/each}
            </div>
          {/if}
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  .lessons-container { display: flex; flex-direction: column; height: calc(100vh - 140px); min-height: 480px; }

  .split-layout {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 24px;
    flex: 1;
    min-height: 0;
  }
  @media (max-width: 1100px) {
    .split-layout { grid-template-columns: 1fr; }
    .lessons-container { height: auto; }
  }
  .column {
    display: flex;
    flex-direction: column;
    min-height: 0;
    border: 1px solid var(--border);
    background: var(--bg-secondary);
  }
  .column-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 14px 18px;
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }
  .column-header .section-label-row { margin-bottom: 0; }
  .column-scroll {
    flex: 1;
    overflow-y: auto;
    padding: 16px 18px;
    min-height: 0;
  }
  .pagination.compact {
    margin: 0;
    padding: 0;
    gap: 6px;
  }
  .pagination.compact .pagination-btn {
    padding: 4px 10px;
    font-size: 12px;
  }
  .pagination.compact .pagination-info {
    font-size: 10px;
    color: var(--text-muted);
    font-family: var(--font-mono);
  }

  /* Search */
  .search-row { margin-bottom: 28px; }
  .search-form { max-width: 400px; }
  .search-input {
    font-family: var(--font-ui);
    font-size: 13px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 0;
    padding: 10px 16px;
    color: var(--text-primary);
    width: 100%;
    transition: border-color 0.2s var(--ease);
  }
  .search-input:focus {
    outline: none;
    border-color: var(--accent);
    box-shadow: 0 0 0 1px var(--accent-muted);
  }
  .search-input::placeholder {
    color: var(--text-muted);
    text-transform: uppercase;
    font-size: 11px;
    letter-spacing: 0.08em;
  }

  /* Section headers */
  .section-label-row {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 20px;
  }
  .section-label-row .gold-line {
    width: 32px;
    height: 2px;
    background: var(--accent);
    margin-bottom: 0;
  }
  .section-label {
    font-family: var(--font-ui);
    font-size: 10px;
    font-weight: 700;
    color: var(--accent);
    text-transform: uppercase;
    letter-spacing: 0.12em;
  }
  .section-count {
    font-family: var(--font-mono);
    font-size: 10px;
    color: var(--text-muted);
  }

  /* Lessons */
  .lessons-list { display: flex; flex-direction: column; gap: 8px; }
  .lesson-card {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: 0;
    padding: 18px 20px;
    transition: border-color 0.2s var(--ease), box-shadow 0.2s var(--ease);
  }
  .lesson-card:hover {
    border-color: var(--accent);
    box-shadow: var(--shadow-hover);
  }
  .lesson-content {
    font-family: var(--font-body);
    font-size: 14px;
    line-height: 1.6;
    color: var(--text-secondary);
  }
  .lesson-meta {
    display: flex;
    align-items: center;
    gap: 16px;
    margin-top: 12px;
  }
  .gauge-row {
    display: flex;
    align-items: center;
    gap: 8px;
    flex: 1;
    max-width: 200px;
  }
  .gauge {
    flex: 1;
    height: 4px;
    background: var(--bg-secondary);
    border-radius: 0;
    overflow: hidden;
  }
  .gauge-fill {
    height: 100%;
    background: var(--accent);
    transition: width 0.4s var(--ease-out);
  }
  .gauge-label {
    font-size: 10px;
    color: var(--text-muted);
    min-width: 28px;
    text-align: right;
  }
  .reinforcement-count {
    font-size: 11px;
    color: var(--text-muted);
    flex-shrink: 0;
  }
  .lesson-dismiss {
    margin-left: auto;
    background: transparent;
    border: 1px solid transparent;
    color: var(--text-muted);
    font-family: var(--font-ui);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    padding: 3px 8px;
    cursor: pointer;
    transition: all 0.15s var(--ease);
  }
  .lesson-dismiss:hover:not(:disabled) {
    color: var(--danger, #ef4444);
    border-color: rgba(239, 68, 68, 0.3);
  }
  .lesson-dismiss:disabled { opacity: 0.5; cursor: not-allowed; }
  .lesson-card-dismissing { opacity: 0.5; }
  .lesson-tags {
    display: flex;
    gap: 6px;
    flex-wrap: wrap;
    margin-top: 10px;
  }
  .tag-badge {
    display: inline-flex;
    align-items: center;
    padding: 2px 10px;
    border: 1px solid rgba(200,147,58,0.25);
    background: var(--accent-muted);
    color: var(--accent);
    font-family: var(--font-ui);
    font-size: 10px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    border-radius: 0;
  }

  /* Insights */
  .insights-list { display: flex; flex-direction: column; gap: 8px; }
  .insight-card {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-left: 2px solid var(--accent);
    border-radius: 0;
    padding: 18px 20px;
    transition: border-color 0.2s var(--ease), box-shadow 0.2s var(--ease);
    text-align: left;
    width: 100%;
    cursor: pointer;
    color: inherit;
    font: inherit;
  }
  .insight-card:hover {
    box-shadow: var(--shadow-hover);
    border-color: var(--accent);
  }
  .insight-card-expanded {
    border-color: var(--accent);
    box-shadow: var(--shadow-hover);
  }
  .insight-card-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
    margin-bottom: 6px;
  }
  .insight-title {
    font-family: var(--font-display);
    font-size: 16px;
    font-weight: 600;
    color: var(--text-primary);
    letter-spacing: -0.02em;
    flex: 1;
    line-height: 1.3;
  }
  .insight-toggle {
    font-size: 16px;
    color: var(--accent);
    flex-shrink: 0;
    line-height: 1;
  }
  .insight-content {
    font-family: var(--font-body);
    font-size: 13px;
    line-height: 1.6;
    color: var(--text-dim);
  }
  .insight-content-collapsed {
    display: -webkit-box;
    -webkit-line-clamp: 3;
    line-clamp: 3;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }
  .insight-meta {
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin-top: 14px;
    padding: 12px 14px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
  }
  .insight-meta-row {
    display: flex;
    align-items: center;
    gap: 12px;
    font-size: 12px;
  }
  .insight-meta-key {
    font-family: var(--font-ui);
    font-size: 10px;
    font-weight: 600;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.08em;
    min-width: 110px;
  }
  .insight-meta-val {
    color: var(--text-secondary);
    font-size: 12px;
  }
  .insight-tags {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 6px;
    margin-top: 12px;
  }
  .insight-tags-label {
    font-family: var(--font-ui);
    font-size: 9px;
    font-weight: 700;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.1em;
    margin-right: 4px;
  }
  .insight-sources {
    display: flex;
    gap: 16px;
    margin-top: 12px;
    padding-top: 10px;
    border-top: 1px solid var(--border);
  }
  .insight-sources-label {
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--text-muted);
  }

  /* Loading */
  .loading-blocks { display: flex; flex-direction: column; gap: 8px; }
  .skeleton-block {
    height: 72px;
    background: var(--bg-card);
    border: 1px solid var(--border);
    animation: pulse 1.5s ease-in-out infinite;
  }
  @keyframes pulse {
    0%, 100% { opacity: 0.4; }
    50% { opacity: 0.8; }
  }
</style>
