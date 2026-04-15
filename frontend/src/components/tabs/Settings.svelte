<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../../lib/api';

  let settings: any = null;
  let loading = true;
  let saving = false;
  let message = '';

  // Form state
  let providerOrder = 'anthropic, openrouter, llamacpp';
  let anthropicModel = '';
  let anthropicApiKey = '';
  let anthropicAuth = '';
  let openrouterModel = '';
  let openrouterApiKey = '';
  let llamacppUrl = '';
  let llamacppModel = '';
  let bm25Weight = 0.4;
  let vectorWeight = 0.4;
  let compressWorkers = 4;
  let consolidationEnabled = true;
  let contextTokenBudget = 2000;

  const anthropicModels = [
    'claude-haiku-4-5-20251001',
    'claude-sonnet-4-6-20250514',
    'claude-opus-4-6-20250514',
    'claude-sonnet-4-20250514',
    'claude-haiku-4-20250414',
  ];

  onMount(async () => {
    try {
      settings = await api.getSettings() as any;
      populateForm(settings);
    } catch (e) { console.error(e); }
    loading = false;
  });

  function populateForm(s: any) {
    if (!s) return;
    const llm = s.llm || {};
    const search = s.search || {};
    const pipeline = s.pipeline || {};

    providerOrder = (llm.providerOrder || []).join(', ');
    anthropicModel = llm.anthropicModel || '';
    anthropicAuth = llm.anthropicAuth || '';
    openrouterModel = llm.openrouterModel || '';
    llamacppUrl = llm.llamacppUrl || '';
    llamacppModel = llm.llamacppModel || '';
    bm25Weight = search.bm25Weight ?? 0.4;
    vectorWeight = search.vectorWeight ?? 0.4;
    compressWorkers = pipeline.compressWorkers ?? 4;
    consolidationEnabled = pipeline.consolidationEnabled ?? true;
    contextTokenBudget = pipeline.contextTokenBudget ?? 2000;
  }

  async function save() {
    saving = true;
    message = '';
    try {
      const order = providerOrder.split(',').map(s => s.trim()).filter(Boolean);
      const body: any = {
        providerOrder: order,
        anthropicModel,
        openrouterModel,
        llamacppUrl,
        llamacppModel,
        bm25Weight,
        vectorWeight,
        compressWorkers,
        consolidationEnabled,
        contextTokenBudget,
      };
      // Only send API keys if user typed new ones (not masked values)
      if (anthropicApiKey && !anthropicApiKey.includes('...')) {
        body.anthropicApiKey = anthropicApiKey;
      }
      if (openrouterApiKey && !openrouterApiKey.includes('...')) {
        body.openrouterApiKey = openrouterApiKey;
      }

      const result = await api.updateSettings(body) as any;
      message = 'Settings saved successfully.';
      if (result.settings) populateForm(result.settings);
    } catch (e: any) {
      message = 'Error: ' + (e.message || e);
    }
    saving = false;
  }
</script>

{#if loading}
  <p style="color:var(--text-muted)">Loading...</p>
{:else}
  <div class="settings-grid">
    <!-- LLM Provider -->
    <div class="card section">
      <h3>LLM Provider</h3>
      <p class="hint">Choose which model processes your observations and generates memories.</p>

      <label class="field">
        <span class="field-label">Provider Priority</span>
        <input class="input" bind:value={providerOrder} placeholder="anthropic, openrouter, llamacpp" />
        <span class="field-hint">Comma-separated. First available provider is used, others are fallbacks.</span>
      </label>

      <div class="provider-section">
        <h4>Anthropic</h4>
        {#if anthropicAuth === 'oauth'}
          <div class="badge badge-success" style="margin-bottom:8px">Claude Code OAuth (auto-detected)</div>
        {/if}
        <label class="field">
          <span class="field-label">Model</span>
          <select class="input" bind:value={anthropicModel}>
            {#each anthropicModels as m}<option value={m}>{m}</option>{/each}
          </select>
        </label>
        <label class="field">
          <span class="field-label">API Key (optional if using OAuth)</span>
          <input class="input mono" type="password" bind:value={anthropicApiKey} placeholder={settings?.llm?.anthropicApiKey || 'Not set'} />
        </label>
      </div>

      <div class="provider-section">
        <h4>OpenRouter</h4>
        <label class="field">
          <span class="field-label">Model</span>
          <input class="input" bind:value={openrouterModel} placeholder="anthropic/claude-haiku-4-5-20251001" />
        </label>
        <label class="field">
          <span class="field-label">API Key</span>
          <input class="input mono" type="password" bind:value={openrouterApiKey} placeholder={settings?.llm?.openrouterApiKey || 'Not set'} />
        </label>
      </div>

      <div class="provider-section">
        <h4>llama.cpp</h4>
        <label class="field">
          <span class="field-label">Server URL</span>
          <input class="input" bind:value={llamacppUrl} placeholder="http://localhost:8080" />
        </label>
        <label class="field">
          <span class="field-label">Model (optional)</span>
          <input class="input" bind:value={llamacppModel} placeholder="Server default" />
        </label>
      </div>
    </div>

    <!-- Search -->
    <div class="card section">
      <h3>Search</h3>
      <label class="field">
        <span class="field-label">BM25 Weight ({bm25Weight})</span>
        <input type="range" min="0" max="1" step="0.1" bind:value={bm25Weight} />
      </label>
      <label class="field">
        <span class="field-label">Vector Weight ({vectorWeight})</span>
        <input type="range" min="0" max="1" step="0.1" bind:value={vectorWeight} />
      </label>
    </div>

    <!-- Pipeline -->
    <div class="card section">
      <h3>Pipeline</h3>
      <label class="field">
        <span class="field-label">Compression Workers</span>
        <input class="input" type="number" min="1" max="16" bind:value={compressWorkers} />
      </label>
      <label class="field">
        <span class="field-label">Context Token Budget</span>
        <input class="input" type="number" min="500" max="10000" step="500" bind:value={contextTokenBudget} />
      </label>
      <label class="field checkbox">
        <input type="checkbox" bind:checked={consolidationEnabled} />
        <span>Enable memory consolidation</span>
      </label>
    </div>
  </div>

  <div style="margin-top:20px;display:flex;align-items:center;gap:12px">
    <button class="btn btn-primary" on:click={save} disabled={saving}>
      {saving ? 'Saving...' : 'Save Settings'}
    </button>
    {#if message}
      <span style="font-size:13px" class:success={!message.startsWith('Error')} class:error={message.startsWith('Error')}>{message}</span>
    {/if}
  </div>
{/if}

<style>
  .settings-grid { display:grid; grid-template-columns:1fr 1fr; gap:20px; }
  @media (max-width: 900px) { .settings-grid { grid-template-columns:1fr; } }
  .section h3 { font-size:16px; margin-bottom:4px; }
  .section h4 { font-size:13px; color:var(--accent); margin:16px 0 8px; padding-top:12px; border-top:1px solid var(--border); }
  .section h4:first-of-type { border-top:none; margin-top:12px; }
  .hint { font-size:12px; color:var(--text-muted); margin-bottom:16px; }
  .field { display:flex; flex-direction:column; gap:4px; margin-bottom:12px; }
  .field-label { font-size:12px; font-weight:600; color:var(--text-secondary); }
  .field-hint { font-size:11px; color:var(--text-muted); }
  .checkbox { flex-direction:row; align-items:center; gap:8px; }
  .checkbox input { width:auto; }
  .provider-section { margin-left:0; }
  select.input { appearance:auto; }
  input[type="range"] { width:100%; accent-color:var(--accent); }
  .success { color:var(--success); }
  .error { color:var(--danger); }
</style>
