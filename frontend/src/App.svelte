<script lang="ts">
  import Header from './components/layout/Header.svelte';
  import TabBar from './components/layout/TabBar.svelte';
  import Dashboard from './components/tabs/Dashboard.svelte';
  import Sessions from './components/tabs/Sessions.svelte';
  import Timeline from './components/tabs/Timeline.svelte';
  import Memories from './components/tabs/Memories.svelte';
  import Graph from './components/tabs/Graph.svelte';
  import Actions from './components/tabs/Actions.svelte';
  import Crystals from './components/tabs/Crystals.svelte';
  import Lessons from './components/tabs/Lessons.svelte';
  import Profile from './components/tabs/Profile.svelte';
  import Activity from './components/tabs/Activity.svelte';
  import Audit from './components/tabs/Audit.svelte';
  import Settings from './components/tabs/Settings.svelte';

  let activeTab = 'dashboard';

  function handleSearch(e: CustomEvent) {
    console.log('Search:', e.detail.query);
  }

  const savedTheme = localStorage.getItem('theme') || 'dark';
  document.documentElement.setAttribute('data-theme', savedTheme);
</script>

<div class="app">
  <Header on:search={handleSearch} />
  <TabBar bind:activeTab />
  <main class="content" class:content-graph={activeTab === 'graph'}>
    {#if activeTab === 'dashboard'}<Dashboard />
    {:else if activeTab === 'sessions'}<Sessions />
    {:else if activeTab === 'timeline'}<Timeline />
    {:else if activeTab === 'memories'}<Memories />
    {:else if activeTab === 'graph'}<Graph />
    {:else if activeTab === 'actions'}<Actions />
    {:else if activeTab === 'crystals'}<Crystals />
    {:else if activeTab === 'lessons'}<Lessons />
    {:else if activeTab === 'profile'}<Profile />
    {:else if activeTab === 'activity'}<Activity />
    {:else if activeTab === 'audit'}<Audit />
    {:else if activeTab === 'settings'}<Settings />
    {/if}
  </main>
</div>

<style>
  .app { display:flex; flex-direction:column; height:100vh; }
  .content { flex:1; overflow-y:auto; padding:24px; }
  .content-graph { padding:16px; }
</style>
