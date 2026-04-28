<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api } from '../../lib/api';

  let lessons: any[] = [];
  let insights: any[] = [];
  let lessonsTotal = 0;
  let insightsTotal = 0;
  let searchQuery = '';
  let searching = false;
  let loading = true;
  let lessonsOffset = 0;
  let insightsOffset = 0;
  const limit = 30;
  let pollTimer: ReturnType<typeof setInterval> | undefined;

  onMount(() => {
    loadAll(true);
    pollTimer = setInterval(() => loadAll(false), 15000);
  });

  onDestroy(() => {
    if (pollTimer) clearInterval(pollTimer);
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

  $: lessonsPage = Math.floor(lessonsOffset / limit) + 1;
  $: lessonsTotalPages = Math.max(1, Math.ceil(lessonsTotal / limit));
  $: insightsPage = Math.floor(insightsOffset / limit) + 1;
  $: insightsTotalPages = Math.max(1, Math.ceil(insightsTotal / limit));

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
</script>

<div class="lessons-container">
  <!-- Search -->
  <div class="search-row">
    <form on:submit|preventDefault={doSearch} class="search-form">
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
            <button class="pagination-btn" on:click={lessonsPrev} disabled={searching || lessonsOffset === 0}>{'\u2190'}</button>
            <span class="pagination-info">{lessonsPage}/{lessonsTotalPages}</span>
            <button class="pagination-btn" on:click={lessonsNext} disabled={searching || lessonsOffset + limit >= lessonsTotal}>{'\u2192'}</button>
          </div>
        </div>

        <div class="column-scroll">
          {#if lessons.length === 0}
            <div class="empty-state" style="padding:32px"><p>No lessons yet</p></div>
          {:else}
            <div class="lessons-list">
              {#each lessons as l}
                <div class="lesson-card">
                  <p class="lesson-content">{l.content}</p>
                  <div class="lesson-meta">
                    <div class="gauge-row">
                      <div class="gauge"><div class="gauge-fill" style="width:{(l.confidence || 0) * 100}%"></div></div>
                      <span class="mono gauge-label">{Math.round((l.confidence || 0) * 100)}%</span>
                    </div>
                    <span class="mono reinforcement-count">{l.reinforcements || 0}x</span>
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
            <button class="pagination-btn" on:click={insightsPrev} disabled={insightsOffset === 0}>{'\u2190'}</button>
            <span class="pagination-info">{insightsPage}/{insightsTotalPages}</span>
            <button class="pagination-btn" on:click={insightsNext} disabled={insightsOffset + limit >= insightsTotal}>{'\u2192'}</button>
          </div>
        </div>

        <div class="column-scroll">
          {#if insights.length === 0}
            <div class="empty-state" style="padding:32px"><p>No insights yet</p></div>
          {:else}
            <div class="insights-list">
              {#each insights as i}
                <div class="insight-card">
                  <h4 class="insight-title">{i.title}</h4>
                  <p class="insight-content">{i.content}</p>
                </div>
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
  }
  .insight-card:hover {
    box-shadow: var(--shadow-hover);
  }
  .insight-title {
    font-family: var(--font-display);
    font-size: 16px;
    font-weight: 600;
    color: var(--text-primary);
    letter-spacing: -0.02em;
    margin-bottom: 6px;
  }
  .insight-content {
    font-family: var(--font-body);
    font-size: 13px;
    line-height: 1.6;
    color: var(--text-dim);
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
