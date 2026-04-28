<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../../lib/api';

  let settings: any = $state(null);
  let loading = $state(true);
  let saving = $state(false);
  let message = $state('');

  // Form state
  let providerOrder = $state('anthropic, openrouter, llamacpp');
  let anthropicModel = $state('');
  let anthropicApiKey = $state('');
  let anthropicAuth = $state('');
  let openrouterModel = $state('');
  let openrouterApiKey = $state('');
  let llamacppUrl = $state('');
  let llamacppModel = $state('');
  let bm25Weight = $state(0.4);
  let vectorWeight = $state(0.4);
  let compressWorkers = $state(4);
  let consolidationEnabled = $state(true);
  let contextTokenBudget = $state(2000);
  let pipelineIntervalMin = $state(5);

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
    pipelineIntervalMin = pipeline.pipelineIntervalMin ?? 5;
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
        pipelineIntervalMin,
      };
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
  <div class="settings-loading">
    {#each Array(3) as _}
      <div class="skeleton-section">
        <div class="skeleton-heading"></div>
        <div class="skeleton-field"></div>
        <div class="skeleton-field short"></div>
      </div>
    {/each}
  </div>
{:else}
  <div class="settings-grid">
    <!-- LLM Provider -->
    <div class="settings-section">
      <div class="section-accent"></div>
      <div class="section-label">Configuration</div>
      <h3 class="section-title">LLM Provider</h3>
      <p class="section-hint">Choose which model processes your observations and generates memories.</p>

      <label class="field">
        <span class="field-label">Provider Priority</span>
        <input class="input" bind:value={providerOrder} placeholder="anthropic, openrouter, llamacpp" />
        <span class="field-hint">Comma-separated. First available provider is used, others are fallbacks.</span>
      </label>

      <!-- Anthropic -->
      <div class="provider-divider"></div>
      <h4 class="provider-heading">Anthropic</h4>
      {#if anthropicAuth === 'oauth'}
        <div class="badge badge-success" style="margin-bottom:12px">Claude Code OAuth (auto-detected)</div>
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

      <!-- OpenRouter -->
      <div class="provider-divider"></div>
      <h4 class="provider-heading">OpenRouter</h4>
      <label class="field">
        <span class="field-label">Model</span>
        <input class="input" bind:value={openrouterModel} placeholder="anthropic/claude-haiku-4-5-20251001" />
      </label>
      <label class="field">
        <span class="field-label">API Key</span>
        <input class="input mono" type="password" bind:value={openrouterApiKey} placeholder={settings?.llm?.openrouterApiKey || 'Not set'} />
      </label>

      <!-- llama.cpp -->
      <div class="provider-divider"></div>
      <h4 class="provider-heading">llama.cpp</h4>
      <label class="field">
        <span class="field-label">Server URL</span>
        <input class="input" bind:value={llamacppUrl} placeholder="http://localhost:8080" />
      </label>
      <label class="field">
        <span class="field-label">Model (optional)</span>
        <input class="input" bind:value={llamacppModel} placeholder="Server default" />
      </label>
    </div>

    <!-- Right column -->
    <div class="settings-column-right">
      <!-- Search -->
      <div class="settings-section">
        <div class="section-accent"></div>
        <div class="section-label">Retrieval</div>
        <h3 class="section-title">Search</h3>

        <label class="field">
          <span class="field-label">BM25 Weight</span>
          <div class="range-row">
            <input type="range" class="range-input" min="0" max="1" step="0.1" bind:value={bm25Weight} />
            <span class="range-val mono">{bm25Weight}</span>
          </div>
        </label>
        <label class="field">
          <span class="field-label">Vector Weight</span>
          <div class="range-row">
            <input type="range" class="range-input" min="0" max="1" step="0.1" bind:value={vectorWeight} />
            <span class="range-val mono">{vectorWeight}</span>
          </div>
        </label>
      </div>

      <!-- Pipeline -->
      <div class="settings-section">
        <div class="section-accent"></div>
        <div class="section-label">Processing</div>
        <h3 class="section-title">Pipeline</h3>

        <label class="field">
          <span class="field-label">Compression Workers</span>
          <input class="input" type="number" min="1" max="16" bind:value={compressWorkers} />
        </label>
        <label class="field">
          <span class="field-label">Context Token Budget</span>
          <input class="input" type="number" min="500" max="10000" step="500" bind:value={contextTokenBudget} />
        </label>
        <label class="field">
          <span class="field-label">Pipeline Interval (minutes)</span>
          <input class="input" type="number" min="0" max="60" step="1" bind:value={pipelineIntervalMin} />
          <span class="field-hint">How often to run summarize + consolidate during active sessions. 0 = disabled.</span>
        </label>
        <label class="field checkbox-field">
          <input type="checkbox" class="checkbox-input" bind:checked={consolidationEnabled} />
          <span class="checkbox-text">Enable memory consolidation</span>
        </label>
      </div>
    </div>
  </div>

  <!-- Save bar -->
  <div class="save-bar">
    <button class="btn-save" onclick={save} disabled={saving}>
      {saving ? 'Saving...' : 'Save Settings'}
    </button>
    {#if message}
      <span class="save-message" class:is-error={message.startsWith('Error')}>{message}</span>
    {/if}
  </div>
{/if}

<style>
  /* Grid layout */
  .settings-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 24px;
    align-items: start;
  }
  @media (max-width: 900px) { .settings-grid { grid-template-columns: 1fr; } }

  .settings-column-right {
    display: flex;
    flex-direction: column;
    gap: 24px;
  }

  /* Section card */
  .settings-section {
    background: var(--bg-card);
    border: 1px solid var(--border);
    padding: 24px;
    position: relative;
    transition: border-color 0.3s var(--ease), box-shadow 0.3s var(--ease);
  }
  .settings-section:hover {
    border-color: var(--accent);
    box-shadow: var(--shadow-hover);
  }
  .section-accent {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    height: 2px;
    background: var(--accent);
  }
  .section-label {
    font-size: 10px;
    font-family: var(--font-ui);
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.12em;
    color: var(--text-muted);
    margin-bottom: 4px;
  }
  .section-title {
    font-family: var(--font-display);
    font-size: 20px;
    font-weight: 700;
    letter-spacing: -0.03em;
    margin-bottom: 4px;
    color: var(--text-primary);
  }
  .section-hint {
    font-size: 12px;
    color: var(--text-muted);
    margin-bottom: 20px;
    line-height: 1.5;
  }

  /* Provider sections */
  .provider-divider {
    width: 40px;
    height: 2px;
    background: var(--accent);
    margin: 20px 0 16px;
    opacity: 0.5;
  }
  .provider-heading {
    font-family: var(--font-ui);
    font-size: 12px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.1em;
    color: var(--accent);
    margin-bottom: 12px;
  }

  /* Fields */
  .field {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin-bottom: 16px;
  }
  .field-label {
    font-size: 11px;
    font-family: var(--font-ui);
    font-weight: 700;
    color: var(--text-dim);
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }
  .field-hint {
    font-size: 11px;
    color: var(--text-muted);
  }

  /* Select styling */
  select.input {
    appearance: none;
    background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 24 24' fill='none' stroke='%236a6a6e' stroke-width='2'%3E%3Cpath d='M6 9l6 6 6-6'/%3E%3C/svg%3E");
    background-repeat: no-repeat;
    background-position: right 12px center;
    padding-right: 32px;
  }

  /* Range slider */
  .range-row {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .range-input {
    flex: 1;
    accent-color: var(--accent);
    height: 4px;
  }
  .range-val {
    font-size: 13px;
    color: var(--text-primary);
    font-weight: 600;
    min-width: 32px;
    text-align: right;
  }

  /* Checkbox */
  .checkbox-field {
    flex-direction: row;
    align-items: center;
    gap: 10px;
  }
  .checkbox-input {
    width: 16px;
    height: 16px;
    accent-color: var(--accent);
    flex-shrink: 0;
  }
  .checkbox-text {
    font-size: 13px;
    color: var(--text-secondary);
    font-weight: 500;
  }

  /* Save bar */
  .save-bar {
    display: flex;
    align-items: center;
    gap: 16px;
    margin-top: 28px;
    padding-top: 24px;
    border-top: 1px solid var(--border);
  }
  .btn-save {
    display: inline-flex;
    align-items: center;
    padding: 10px 28px;
    background: var(--accent);
    color: #030303;
    border: 1px solid var(--accent);
    border-radius: 0;
    font-family: var(--font-ui);
    font-size: 13px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    cursor: pointer;
    transition: all 0.2s var(--ease);
  }
  .btn-save:hover {
    background: var(--accent-hover);
    border-color: var(--accent-hover);
  }
  .btn-save:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
  .save-message {
    font-size: 13px;
    color: var(--success);
  }
  .save-message.is-error {
    color: var(--danger);
  }

  /* Loading skeleton */
  .settings-loading {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 24px;
  }
  @media (max-width: 900px) { .settings-loading { grid-template-columns: 1fr; } }
  .skeleton-section {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-top: 2px solid var(--border-hover);
    padding: 24px;
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  .skeleton-heading {
    width: 50%;
    height: 20px;
    background: var(--bg-hover);
    animation: pulse 1.5s ease-in-out infinite;
  }
  .skeleton-field {
    width: 100%;
    height: 36px;
    background: var(--bg-hover);
    animation: pulse 1.5s ease-in-out infinite;
  }
  .skeleton-field.short { width: 60%; }
  @keyframes pulse {
    0%, 100% { opacity: 0.3; }
    50% { opacity: 0.7; }
  }
</style>
