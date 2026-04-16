<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../../lib/api';

  let lessons: any[] = [];
  let insights: any[] = [];
  let searchQuery = '';
  let loading = true;
  let lessonsOffset = 0;
  let insightsOffset = 0;
  const limit = 30;

  onMount(() => loadAll());

  async function loadAll() {
    loading = true;
    try {
      const [l, i] = await Promise.all([
        api.listLessons(limit, lessonsOffset),
        api.listInsights(limit, insightsOffset),
      ]);
      lessons = l.lessons || [];
      insights = i.insights || [];
    } catch(e) { console.error(e); }
    loading = false;
  }

  async function loadLessons() {
    try { const r = await api.listLessons(limit, lessonsOffset); lessons = r.lessons || []; } catch(e) { console.error(e); }
  }
  async function loadInsights() {
    try { const r = await api.listInsights(limit, insightsOffset); insights = r.insights || []; } catch(e) { console.error(e); }
  }

  function lessonsPrev() { if (lessonsOffset >= limit) { lessonsOffset -= limit; loadLessons(); } }
  function lessonsNext() { if (lessons.length >= limit) { lessonsOffset += limit; loadLessons(); } }
  function insightsPrev() { if (insightsOffset >= limit) { insightsOffset -= limit; loadInsights(); } }
  function insightsNext() { if (insights.length >= limit) { insightsOffset += limit; loadInsights(); } }

  $: lessonsPage = Math.floor(lessonsOffset / limit) + 1;
  $: lessonsTotalPages = lessons.length < limit ? lessonsPage : lessonsPage + 1;
  $: insightsPage = Math.floor(insightsOffset / limit) + 1;
  $: insightsTotalPages = insights.length < limit ? insightsPage : insightsPage + 1;

  async function doSearch() {
    if (!searchQuery.trim()) return;
    lessonsOffset = 0;
    try {
      const r = await api.searchLessons(searchQuery);
      lessons = r.lessons || [];
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
    <!-- Lessons Section -->
    <div class="section">
      <div class="section-label-row">
        <div class="gold-line"></div>
        <span class="section-label">LESSONS</span>
        <span class="section-count">{lessons.length}</span>
      </div>

      {#if lessons.length === 0}
        <div class="empty-state" style="padding:32px">
          <p>No lessons yet</p>
        </div>
      {:else}
        <div class="lessons-list">
          {#each lessons as l}
            <div class="lesson-card">
              <p class="lesson-content">{l.content}</p>
              <div class="lesson-meta">
                <div class="gauge-row">
                  <div class="gauge">
                    <div class="gauge-fill" style="width:{(l.confidence || 0) * 100}%"></div>
                  </div>
                  <span class="mono gauge-label">{Math.round((l.confidence || 0) * 100)}%</span>
                </div>
                <span class="mono reinforcement-count">{l.reinforcements || 0}x</span>
              </div>
              {#if parseTags(l.tags).length > 0}
                <div class="lesson-tags">
                  {#each parseTags(l.tags) as t}
                    <span class="tag-badge">{t}</span>
                  {/each}
                </div>
              {/if}
            </div>
          {/each}
        </div>
        <div class="pagination">
          <button class="pagination-btn" on:click={lessonsPrev} disabled={lessonsOffset === 0}>{'\u2190'} PREV</button>
          <span class="pagination-info">PAGE {lessonsPage} OF {lessonsTotalPages}</span>
          <button class="pagination-btn" on:click={lessonsNext} disabled={lessons.length < limit}>NEXT {'\u2192'}</button>
        </div>
      {/if}
    </div>

    <!-- Insights Section -->
    <div class="section" style="margin-top:40px">
      <div class="section-label-row">
        <div class="gold-line"></div>
        <span class="section-label">INSIGHTS</span>
        <span class="section-count">{insights.length}</span>
      </div>

      {#if insights.length === 0}
        <div class="empty-state" style="padding:32px">
          <p>No insights yet</p>
        </div>
      {:else}
        <div class="insights-list">
          {#each insights as i}
            <div class="insight-card">
              <h4 class="insight-title">{i.title}</h4>
              <p class="insight-content">{i.content}</p>
            </div>
          {/each}
        </div>
        <div class="pagination">
          <button class="pagination-btn" on:click={insightsPrev} disabled={insightsOffset === 0}>{'\u2190'} PREV</button>
          <span class="pagination-info">PAGE {insightsPage} OF {insightsTotalPages}</span>
          <button class="pagination-btn" on:click={insightsNext} disabled={insights.length < limit}>NEXT {'\u2192'}</button>
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .lessons-container { display: flex; flex-direction: column; }

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
