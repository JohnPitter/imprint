<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  // Component event dispatchers are gone in Svelte 5 — parents pass callbacks
  // as plain props. App.svelte calls us with onsearch={...}.
  let { onsearch }: { onsearch?: (detail: { query: string }) => void } = $props();

  let searchQuery = $state('');
  let theme = $state(localStorage.getItem('theme') || 'dark');
  let searchInput: HTMLInputElement | undefined = $state(undefined);

  function toggleTheme() {
    theme = theme === 'dark' ? 'light' : 'dark';
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem('theme', theme);
  }

  function submitSearch(e: SubmitEvent) {
    e.preventDefault();
    if (searchQuery.trim()) onsearch?.({ query: searchQuery });
  }

  // Atalho global "/" foca o search — mesmo padrão do GitHub/Linear.
  // Ignora se o user já está digitando em input/textarea pra não
  // capturar quando ele queria literalmente um "/" no campo.
  function onGlobalKeydown(e: KeyboardEvent) {
    if (e.key !== '/') return;
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    const target = e.target as HTMLElement | null;
    if (target) {
      const tag = target.tagName.toUpperCase();
      if (tag === 'INPUT' || tag === 'TEXTAREA' || target.isContentEditable) return;
    }
    e.preventDefault();
    searchInput?.focus();
    searchInput?.select();
  }

  onMount(() => window.addEventListener('keydown', onGlobalKeydown));
  onDestroy(() => window.removeEventListener('keydown', onGlobalKeydown));
</script>

<header class="header">
  <div class="left"><h1 class="logo">Imprint</h1></div>
  <div class="center">
    <form onsubmit={submitSearch} class="search-wrap">
      <input
        bind:this={searchInput}
        class="input"
        bind:value={searchQuery}
        placeholder="Search memories, observations..."
      />
      <kbd class="search-kbd">/</kbd>
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
  /* Search wrapper com kbd hint à direita — só aparece quando o input
     não está focado, sumindo na hora que o user vai digitar. */
  .search-wrap { position: relative; }
  .search-wrap .input { padding-right: 32px; }
  .search-kbd {
    position: absolute;
    right: 8px; top: 50%;
    transform: translateY(-50%);
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--text-muted);
    background: var(--bg-card);
    border: 1px solid var(--border);
    padding: 1px 6px;
    pointer-events: none;
    transition: opacity 0.15s ease;
  }
  .search-wrap:focus-within .search-kbd { opacity: 0; }
</style>
