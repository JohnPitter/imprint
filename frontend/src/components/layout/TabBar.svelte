<script lang="ts">
  export let activeTab: string = 'dashboard';
  const tabs = [
    { id: 'dashboard', label: 'Dashboard' },
    { id: 'sessions', label: 'Sessions' },
    { id: 'timeline', label: 'Timeline' },
    { id: 'memories', label: 'Memories' },
    { id: 'graph', label: 'Graph' },
    { id: 'actions', label: 'Actions' },
    { id: 'crystals', label: 'Crystals' },
    { id: 'lessons', label: 'Lessons' },
    { id: 'profile', label: 'Profile' },
    { id: 'activity', label: 'Activity' },
    { id: 'audit', label: 'Audit' },
    { id: 'settings', label: 'Settings' },
  ];
</script>

<nav class="tabbar">
  {#each tabs as tab}
    <button class="tab" class:active={activeTab === tab.id} on:click={() => activeTab = tab.id}>
      {tab.label}
    </button>
  {/each}
</nav>

<style>
  .tabbar { display:flex; overflow-x:auto; gap:2px; padding:0 16px; background:var(--bg-secondary); border-bottom:1px solid var(--border); }
  .tab { display:flex; align-items:center; padding:10px 14px; border:none; background:transparent; color:var(--text-secondary); font-size:13px; font-weight:500; cursor:pointer; border-bottom:2px solid transparent; transition:all 0.15s; white-space:nowrap; }
  .tab:hover { color:var(--text-primary); background:var(--bg-hover); }
  .tab.active { color:var(--accent); border-bottom-color:var(--accent); }
</style>
