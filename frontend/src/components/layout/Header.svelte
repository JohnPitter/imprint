<script lang="ts">
  // Component event dispatchers are gone in Svelte 5 — parents pass callbacks
  // as plain props. App.svelte calls us with onsearch={...}.
  let { onsearch }: { onsearch?: (detail: { query: string }) => void } = $props();

  let searchQuery = $state('');
  let theme = $state(localStorage.getItem('theme') || 'dark');

  function toggleTheme() {
    theme = theme === 'dark' ? 'light' : 'dark';
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem('theme', theme);
  }

  function submitSearch(e: SubmitEvent) {
    e.preventDefault();
    if (searchQuery.trim()) onsearch?.({ query: searchQuery });
  }
</script>

<header class="header">
  <div class="left"><h1 class="logo">Imprint</h1></div>
  <div class="center">
    <form onsubmit={submitSearch}>
      <input class="input" bind:value={searchQuery} placeholder="Search memories, observations..." />
    </form>
  </div>
  <div class="right">
    <button class="toggle" onclick={toggleTheme}>{theme === 'dark' ? '☀' : '☾'}</button>
  </div>
</header>

<style>
  .header { display:flex; align-items:center; justify-content:space-between; padding:0 28px; height:56px; background:var(--bg-secondary); border-bottom:1px solid var(--border); }
  .logo { font-size:20px; font-weight:700; color:var(--accent); letter-spacing:-0.03em; font-family:var(--font-display); }
  .center { flex:1; max-width:480px; margin:0 28px; }
  .toggle { padding:6px 10px; font-size:16px; background:transparent; border:none; cursor:pointer; color:var(--text-dim); transition:color 0.2s; }
  .toggle:hover { color:var(--accent); }
</style>
