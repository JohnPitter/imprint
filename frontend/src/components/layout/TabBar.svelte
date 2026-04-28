<script lang="ts">
  // bindable so the parent's `bind:activeTab` keeps working in runes mode.
  let { activeTab = $bindable('dashboard') }: { activeTab?: string } = $props();
  const tabs = [
    { id: 'dashboard', label: 'Dashboard' },
    { id: 'sessions', label: 'Sessions' },
    { id: 'timeline', label: 'Timeline' },
    { id: 'memories', label: 'Memories' },
    { id: 'graph', label: 'Graph' },
    { id: 'actions', label: 'Actions' },
    { id: 'lessons', label: 'Lessons' },
    { id: 'profile', label: 'Profile' },
    { id: 'activity', label: 'Activity' },
    { id: 'audit', label: 'Audit' },
    { id: 'settings', label: 'Settings' },
  ];
</script>

<nav class="tabbar">
  {#each tabs as tab}
    <button class="tab" class:active={activeTab === tab.id} onclick={() => activeTab = tab.id}>
      {tab.label}
    </button>
  {/each}
</nav>

<style>
  .tabbar { display:flex; overflow-x:auto; gap:0; padding:0 20px; background:var(--bg-secondary); border-bottom:1px solid var(--border); }
  .tab { display:flex; align-items:center; padding:12px 16px; border:none; background:transparent; color:var(--text-muted); font-size:11px; font-weight:700; font-family:var(--font-ui); cursor:pointer; border-bottom:2px solid transparent; transition:all 0.2s; white-space:nowrap; text-transform:uppercase; letter-spacing:0.08em; }
  .tab:hover { color:var(--text-primary); }
  .tab.active { color:var(--accent); border-bottom-color:var(--accent); }
</style>
