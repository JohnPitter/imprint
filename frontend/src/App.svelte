<script lang="ts">
  import Header from './components/layout/Header.svelte';
  import TabBar from './components/layout/TabBar.svelte';
  import Dashboard from './components/tabs/Dashboard.svelte';
  import Recall from './components/tabs/Recall.svelte';
  import Sessions from './components/tabs/Sessions.svelte';
  import Timeline from './components/tabs/Timeline.svelte';
  import Memories from './components/tabs/Memories.svelte';
  import Graph from './components/tabs/Graph.svelte';
  import Actions from './components/tabs/Actions.svelte';
  import Lessons from './components/tabs/Lessons.svelte';
  import Profile from './components/tabs/Profile.svelte';
  import Activity from './components/tabs/Activity.svelte';
  import Audit from './components/tabs/Audit.svelte';
  import Settings from './components/tabs/Settings.svelte';
  import { api } from './lib/api';

  let activeTab = $state('dashboard');
  let searchOpen = $state(false);
  let searchQuery = $state('');
  let searchLoading = $state(false);
  let searchResults: any[] = $state([]);
  let searchError = $state('');

  // Search filters: subset of types to show, plus a minimum-score floor.
  // Score in the API is roughly RRF-normalized to ~0.005..0.020; default the
  // floor at 0 so we don't hide results, but expose the slider.
  let searchTypeFilter: Set<string> = $state(new Set());
  let searchMinScore = $state(0);

  let searchTypeCounts = $derived(((): Record<string, number> => {
    const out: Record<string, number> = {};
    for (const r of searchResults) {
      const t = (r.type || 'note').toLowerCase();
      out[t] = (out[t] || 0) + 1;
    }
    return out;
  })());

  let visibleResults = $derived(searchResults.filter((r) => {
    if (searchMinScore > 0 && (r.score || 0) * 1000 < searchMinScore) return false;
    if (searchTypeFilter.size === 0) return true;
    const t = (r.type || 'note').toLowerCase();
    return searchTypeFilter.has(t);
  }));

  function toggleSearchType(t: string) {
    if (searchTypeFilter.has(t)) searchTypeFilter.delete(t);
    else searchTypeFilter.add(t);
    searchTypeFilter = new Set(searchTypeFilter);
  }
  function clearSearchFilters() {
    searchTypeFilter = new Set();
    searchMinScore = 0;
  }

  // Header now passes a plain callback prop instead of dispatching an event.
  async function handleSearch(detail: { query: string }) {
    const q = (detail.query || '').trim();
    if (!q) return;
    searchQuery = q;
    searchOpen = true;
    searchLoading = true;
    searchError = '';
    searchResults = [];
    clearSearchFilters();
    try {
      const r: any = await api.search(q, 25);
      searchResults = r.results || [];
    } catch (err: any) {
      searchError = err?.message || 'Search failed';
    } finally {
      searchLoading = false;
    }
  }

  function closeSearch() {
    searchOpen = false;
  }

  function truncate(s: string | undefined, n: number): string {
    if (!s) return '';
    return s.length > n ? s.slice(0, n - 1) + '…' : s;
  }

  const savedTheme = localStorage.getItem('theme') || 'dark';
  document.documentElement.setAttribute('data-theme', savedTheme);
</script>

<div class="app">
  <Header onsearch={handleSearch} />
  <TabBar bind:activeTab />
  <main class="content" class:content-graph={activeTab === 'graph'}>
    {#if activeTab === 'dashboard'}<Dashboard />
    {:else if activeTab === 'recall'}<Recall />
    {:else if activeTab === 'sessions'}<Sessions />
    {:else if activeTab === 'timeline'}<Timeline />
    {:else if activeTab === 'memories'}<Memories />
    {:else if activeTab === 'graph'}<Graph />
    {:else if activeTab === 'actions'}<Actions />
    {:else if activeTab === 'lessons'}<Lessons />
    {:else if activeTab === 'profile'}<Profile />
    {:else if activeTab === 'activity'}<Activity />
    {:else if activeTab === 'audit'}<Audit />
    {:else if activeTab === 'settings'}<Settings />
    {/if}
  </main>

  {#if searchOpen}
    <div class="search-overlay" onclick={closeSearch} onkeydown={(e) => e.key === 'Escape' && closeSearch()} role="presentation">
      <div class="search-panel" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()} role="dialog" aria-label="Search results" tabindex="-1">
        <div class="search-panel-header">
          <span class="search-panel-label">RESULTS FOR</span>
          <span class="search-panel-query">{searchQuery}</span>
          <span class="search-panel-count">{visibleResults.length}{#if searchResults.length !== visibleResults.length} / {searchResults.length}{/if}</span>
          <button class="search-panel-close" onclick={closeSearch} aria-label="Close">×</button>
        </div>

        {#if searchResults.length > 0}
          <div class="search-filters">
            <div class="search-filter-chips">
              {#each Object.entries(searchTypeCounts) as [t, n]}
                <button
                  class="search-filter-chip"
                  class:search-filter-chip-active={searchTypeFilter.has(t)}
                  onclick={() => toggleSearchType(t)}
                >
                  {t} <span class="search-filter-chip-count">{n}</span>
                </button>
              {/each}
              {#if searchTypeFilter.size > 0 || searchMinScore > 0}
                <button class="search-filter-clear" onclick={clearSearchFilters}>clear</button>
              {/if}
            </div>
            <label class="search-filter-score">
              <span class="search-filter-score-label">MIN SCORE</span>
              <input type="range" min="0" max="20" step="1" bind:value={searchMinScore} />
              <span class="search-filter-score-val mono">{searchMinScore}</span>
            </label>
          </div>
        {/if}

        <div class="search-panel-body">
          {#if searchLoading}
            <div class="search-empty">Searching…</div>
          {:else if searchError}
            <div class="search-empty search-error">{searchError}</div>
          {:else if searchResults.length === 0}
            <div class="search-empty">No matches for "{searchQuery}"</div>
          {:else if visibleResults.length === 0}
            <div class="search-empty">All {searchResults.length} results filtered out — clear filters to see them.</div>
          {:else}
            <ul class="search-results">
              {#each visibleResults as r}
                <li class="search-result">
                  <div class="search-result-head">
                    <span class="search-result-type">{r.type || 'note'}</span>
                    <span class="search-result-title">{r.title || '(untitled)'}</span>
                    <span class="search-result-score">{(r.score * 1000).toFixed(0)}</span>
                  </div>
                  {#if r.narrative}
                    <p class="search-result-narrative">{truncate(r.narrative, 220)}</p>
                  {/if}
                  {#if (r.concepts && r.concepts.length) || (r.files && r.files.length)}
                    <div class="search-result-tags">
                      {#each (r.concepts || []).slice(0, 5) as c}
                        <span class="search-result-tag">{c}</span>
                      {/each}
                      {#each (r.files || []).slice(0, 3) as f}
                        <span class="search-result-tag search-result-tag-file">{f}</span>
                      {/each}
                    </div>
                  {/if}
                </li>
              {/each}
            </ul>
          {/if}
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  .app { display:flex; flex-direction:column; height:100vh; }
  .content { flex:1; overflow-y:auto; padding:24px; }
  .content-graph { padding:16px; }

  .search-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0,0,0,0.45);
    z-index: 1000;
    display: flex;
    justify-content: center;
    align-items: flex-start;
    padding-top: 80px;
  }
  .search-panel {
    width: min(720px, calc(100vw - 32px));
    max-height: calc(100vh - 120px);
    display: flex;
    flex-direction: column;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    box-shadow: 0 12px 40px rgba(0,0,0,0.4);
  }
  .search-panel-header {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 14px 18px;
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }
  .search-panel-label {
    font-family: var(--font-mono);
    font-size: 10px;
    letter-spacing: 0.12em;
    color: var(--text-muted);
    text-transform: uppercase;
  }
  .search-panel-query {
    flex: 1;
    font-family: var(--font-body);
    font-size: 14px;
    color: var(--text-primary);
    font-weight: 500;
  }
  .search-panel-count {
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--accent);
  }
  .search-panel-close {
    background: transparent;
    border: none;
    font-size: 22px;
    line-height: 1;
    color: var(--text-muted);
    cursor: pointer;
    padding: 0 4px;
  }
  .search-panel-close:hover { color: var(--text-primary); }
  .search-panel-body {
    flex: 1;
    overflow-y: auto;
    padding: 8px 0;
  }

  /* Filters bar */
  .search-filters {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 10px 18px;
    border-bottom: 1px solid var(--border);
    flex-wrap: wrap;
  }
  .search-filter-chips {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 4px;
  }
  .search-filter-chip {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 4px 10px;
    background: transparent;
    border: 1px solid var(--border);
    color: var(--text-muted);
    font-family: var(--font-ui);
    font-size: 10px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    cursor: pointer;
    transition: all 0.15s var(--ease);
  }
  .search-filter-chip:hover { color: var(--text-primary); border-color: var(--text-dim); }
  .search-filter-chip-active {
    color: var(--accent);
    border-color: var(--accent);
    background: var(--accent-muted);
  }
  .search-filter-chip-count {
    font-family: var(--font-mono);
    font-size: 9px;
    opacity: 0.7;
  }
  .search-filter-clear {
    margin-left: 6px;
    background: transparent;
    border: none;
    color: var(--text-muted);
    font-family: var(--font-ui);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    cursor: pointer;
    padding: 4px 6px;
  }
  .search-filter-clear:hover { color: var(--accent); }
  .search-filter-score {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .search-filter-score-label {
    font-family: var(--font-ui);
    font-size: 10px;
    font-weight: 700;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }
  .search-filter-score input[type="range"] {
    width: 100px;
    accent-color: var(--accent);
    height: 4px;
  }
  .search-filter-score-val {
    font-size: 11px;
    color: var(--text-secondary);
    min-width: 18px;
    text-align: right;
  }
  .search-empty {
    padding: 32px;
    text-align: center;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--text-muted);
  }
  .search-error { color: var(--danger, #ef4444); }
  .search-results {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .search-result {
    padding: 14px 18px;
    border-bottom: 1px solid var(--border);
  }
  .search-result:last-child { border-bottom: none; }
  .search-result-head {
    display: flex;
    align-items: baseline;
    gap: 10px;
    margin-bottom: 6px;
  }
  .search-result-type {
    font-family: var(--font-mono);
    font-size: 9px;
    text-transform: uppercase;
    letter-spacing: 0.1em;
    color: var(--accent);
    border: 1px solid var(--accent);
    padding: 2px 6px;
    flex-shrink: 0;
  }
  .search-result-title {
    flex: 1;
    font-family: var(--font-body);
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
  }
  .search-result-score {
    font-family: var(--font-mono);
    font-size: 10px;
    color: var(--text-muted);
  }
  .search-result-narrative {
    font-size: 13px;
    line-height: 1.5;
    color: var(--text-dim);
    margin: 6px 0 0 0;
  }
  .search-result-tags {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-top: 8px;
  }
  .search-result-tag {
    font-family: var(--font-mono);
    font-size: 10px;
    color: var(--text-muted);
    background: var(--bg-card);
    border: 1px solid var(--border);
    padding: 2px 6px;
  }
  .search-result-tag-file {
    color: var(--text-dim);
    font-style: italic;
  }
</style>
