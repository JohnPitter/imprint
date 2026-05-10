<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api } from '../../lib/api';
  import { createPoller } from '../../lib/poller';
  import { truncate } from '../../lib/format';
  import { getURLState, setURLState } from '../../lib/urlState';
  import ConceptTag from '../ConceptTag.svelte';

  let memories: any[] = $state([]);
  let memoriesTotal = $state(0);
  let filter = $state('');
  let loading = $state(true);
  let offset = $state(0);
  const limit = 30;
  const types = ['', 'pattern', 'preference', 'architecture', 'bug', 'workflow', 'fact'];
  const editableTypes = types.filter(t => t !== '');
  let stopPoll: (() => void) | undefined;

  // Time-travel: 0 = hoje, 1 = 1 dia atrás, ... 365 = 1 ano. O backend
  // filtra created_at <= cutoff. Note: como o schema mantém só is_latest=1,
  // isso mostra "memórias que existiam até essa data", não snapshot exato.
  let daysAgo = $state(0);
  let beforeISO = $derived.by(() => {
    if (daysAgo <= 0) return '';
    const d = new Date(Date.now() - daysAgo * 24 * 60 * 60 * 1000);
    return d.toISOString();
  });
  let beforeLabel = $derived.by(() => {
    if (daysAgo <= 0) return 'now';
    if (daysAgo === 1) return '1 day ago';
    if (daysAgo < 7) return `${daysAgo} days ago`;
    if (daysAgo < 30) return `${Math.round(daysAgo / 7)}w ago`;
    if (daysAgo < 365) return `${Math.round(daysAgo / 30)}mo ago`;
    return `${(daysAgo / 365).toFixed(1)}y ago`;
  });

  // Modal state
  let editing: any = $state(null);
  let editTitle = $state('');
  let editContent = $state('');
  let editType = $state('');
  let editStrength = $state(5);
  let editConcepts: string[] = $state([]);
  let newConcept = $state('');
  let saving = $state(false);
  let pinning = $state(false);
  let confirmingForget = $state(false);
  let modalError = $state('');
  // History panel within the modal: empty until lazy-loaded.
  let history: any[] = $state([]);
  let historyLoading = $state(false);
  let historyExpanded = $state(false);
  // "Why is this in my context?" — score breakdown vindo do hybrid search.
  let whyResults: any[] = $state([]);
  let whyLoading = $state(false);
  let whyExpanded = $state(false);

  onMount(() => {
    // Hidrata estado do URL: ?type=pattern&days=7 sobrevive a refresh.
    const url = getURLState();
    if (url.type) filter = url.type;
    const d = parseInt(url.days || '0', 10);
    if (!isNaN(d) && d > 0) daysAgo = Math.min(365, d);
    load(true);
    stopPoll = createPoller(() => load(false), 15000);
  });

  // Reflete mudanças de filter/daysAgo no URL pra refresh-resistant + share.
  $effect(() => {
    setURLState({ type: filter || '', days: daysAgo || 0 });
  });

  onDestroy(() => {
    stopPoll?.();
  });

  async function load(initial: boolean) {
    if (initial) loading = true;
    try {
      const r: any = await api.listMemories(filter, limit, offset, beforeISO);
      memories = r.memories || [];
      memoriesTotal = r.total ?? memories.length;
    } catch(e) { console.error(e); }
    if (initial) loading = false;
  }

  function setFilter(t: string) { filter = t; offset = 0; load(true); }
  function setDaysAgo(n: number) { daysAgo = Math.max(0, n); offset = 0; load(true); }
  function prev() { if (offset >= limit) { offset -= limit; load(true); } }
  function next() { if (offset + limit < memoriesTotal) { offset += limit; load(true); } }

  let currentPage = $derived(Math.floor(offset / limit) + 1);
  let totalPages = $derived(Math.max(1, Math.ceil(memoriesTotal / limit)));

  const typeColors: Record<string, string> = {
    pattern: 'badge-info', preference: 'badge-purple', architecture: 'badge-accent',
    bug: 'badge-danger', workflow: 'badge-success', fact: 'badge-warning',
  };

  function openEdit(m: any) {
    editing = m;
    editTitle = m.title || '';
    editContent = m.content || '';
    editType = m.type || 'fact';
    editStrength = m.strength ?? 5;
    editConcepts = parseConceptList(m.concepts);
    newConcept = '';
    confirmingForget = false;
    modalError = '';
    history = [];
    historyExpanded = false;
    whyResults = [];
    whyExpanded = false;
  }

  function parseConceptList(c: any): string[] {
    if (!c) return [];
    if (Array.isArray(c)) return c.filter((s: any) => typeof s === 'string');
    if (typeof c === 'string') {
      try { const p = JSON.parse(c); return Array.isArray(p) ? p : []; } catch { return []; }
    }
    return [];
  }

  // Toggle pin (anti-decay). Otimista no estado local, com rollback em erro.
  async function togglePin() {
    if (!editing || pinning) return;
    pinning = true;
    const target = !(editing.pinned > 0);
    try {
      await api.pinMemory(editing.id, target);
      editing = { ...editing, pinned: target ? 1 : 0 };
      // Reflete na lista também
      memories = memories.map((m: any) => m.id === editing.id ? { ...m, pinned: target ? 1 : 0 } : m);
    } catch (e: any) {
      modalError = 'Falha ao alternar pin: ' + (e?.message || e);
    }
    pinning = false;
  }

  // Concepts inline editor: persiste imediato no backend pra evitar perda
  // se o user fechar o modal sem clicar Save (que apenas evolve title/content).
  async function persistConcepts(next: string[]) {
    if (!editing) return;
    try {
      await api.setMemoryConcepts(editing.id, next);
      editConcepts = next;
      editing = { ...editing, concepts: JSON.stringify(next) };
      memories = memories.map((m: any) => m.id === editing.id ? { ...m, concepts: editing.concepts } : m);
    } catch (e: any) {
      modalError = 'Falha ao salvar concepts: ' + (e?.message || e);
    }
  }
  async function removeConcept(c: string) {
    await persistConcepts(editConcepts.filter(x => x !== c));
  }
  async function addConcept() {
    const c = newConcept.trim();
    if (!c) return;
    if (editConcepts.includes(c)) { newConcept = ''; return; }
    await persistConcepts([...editConcepts, c]);
    newConcept = '';
  }

  // "Why this memory in my context?" — chama search com o título da
  // memória e mostra os top 5 results com scores. Útil pra debugar
  // retrieval (entender quais memórias rivalizam por slot no context).
  async function loadWhy() {
    if (!editing || whyLoading) return;
    whyExpanded = true;
    if (whyResults.length > 0) return; // cache simples por modal aberto
    whyLoading = true;
    try {
      const r = await api.search(editing.title, 5) as any;
      whyResults = r.results || [];
    } catch (e: any) {
      modalError = 'Falha no why-search: ' + (e?.message || e);
    }
    whyLoading = false;
  }

  function closeModal() {
    if (saving) return;
    editing = null;
    confirmingForget = false;
    modalError = '';
    history = [];
    historyExpanded = false;
  }

  async function toggleHistory() {
    if (!editing) return;
    if (historyExpanded) { historyExpanded = false; return; }
    historyExpanded = true;
    if (history.length === 0) {
      historyLoading = true;
      try {
        const r = await api.memoryHistory(editing.id) as any;
        history = r.versions || [];
      } catch (e) {
        // Surface in the modal error band — small failure, not blocking.
        modalError = (e as any)?.message || 'Failed to load history';
      }
      historyLoading = false;
    }
  }

  function fmtTime(ts: string): string {
    if (!ts) return '';
    try { return new Date(ts).toLocaleString(); } catch { return ts; }
  }

  async function saveEdit() {
    if (!editing || saving) return;
    saving = true;
    modalError = '';
    try {
      await api.evolve({
        id: editing.id,
        title: editTitle,
        content: editContent,
        type: editType,
        strength: editStrength,
      });
      // Optimistic update — reload to pick up the new version (the old row is
      // marked is_latest=0 and won't show in /memories anymore).
      await load(false);
      editing = null;
    } catch (e: any) {
      modalError = e?.message || 'Failed to save';
    }
    saving = false;
  }

  async function confirmForget() {
    if (!editing || saving) return;
    saving = true;
    modalError = '';
    try {
      await api.forget({ id: editing.id });
      await load(false);
      editing = null;
    } catch (e: any) {
      modalError = e?.message || 'Failed to forget';
    }
    saving = false;
  }

  function onModalKey(e: KeyboardEvent) {
    if (e.key === 'Escape') closeModal();
  }
</script>

<svelte:window onkeydown={onModalKey} />

<!-- Time-travel slider: arrasta pra ver memórias que existiam em um
     ponto passado. Útil pra "o que eu sabia antes do projeto X começar?".
     Modo "now" = comportamento padrão, sem filtro de data. -->
<div class="time-travel-bar">
  <span class="tt-label">SHOW MEMORIES AS OF</span>
  <span class="tt-value">{beforeLabel}</span>
  <input
    type="range"
    class="tt-slider"
    min="0"
    max="365"
    step="1"
    bind:value={daysAgo}
    oninput={() => { offset = 0; }}
    onchange={() => load(true)}
  />
  {#if daysAgo > 0}
    <button class="tt-reset" onclick={() => setDaysAgo(0)}>back to now</button>
  {/if}
</div>

<!-- Type filter bar -->
<div class="filter-bar">
  {#each types as t}
    <button
      class="filter-btn"
      class:active={filter === t}
      onclick={() => setFilter(t)}
    >{t || 'ALL'}</button>
  {/each}
</div>

{#if loading}
  <div class="loading-state">
    {#each Array(3) as _}
      <div class="skeleton-card">
        <div class="skeleton-line wide"></div>
        <div class="skeleton-line"></div>
        <div class="skeleton-line narrow"></div>
      </div>
    {/each}
  </div>
{:else if memories.length === 0}
  <div class="empty-state">
    <div class="icon">--</div>
    <p>No memories found</p>
  </div>
{:else}
  <div class="memory-list">
    {#each memories as m}
      <button class="memory-card" class:memory-card-pinned={m.pinned > 0} onclick={() => openEdit(m)}>
        <div class="card-header">
          <div class="card-header-left">
            {#if m.pinned > 0}<span class="pin-star" title="Pinned (anti-decay)">★</span>{/if}
            <span class="badge {typeColors[m.type] || 'badge-info'}">{m.type}</span>
            <h4 class="memory-title">{m.title}</h4>
          </div>
          <span class="version-label mono">v{m.version}</span>
        </div>

        <p class="memory-content">{truncate(m.content, 200)}</p>

        <div class="memory-meta">
          <div class="strength-row">
            <span class="strength-label">Strength</span>
            <div class="gauge">
              <div class="gauge-fill" style="width:{m.strength * 10}%"></div>
            </div>
            <span class="strength-val mono">{m.strength}/10</span>
          </div>

          {#if m.concepts?.length > 0}
            <div class="concepts">
              {#each (typeof m.concepts === 'string' ? JSON.parse(m.concepts) : m.concepts) as c}
                <ConceptTag label={c} />
              {/each}
            </div>
          {/if}
        </div>
      </button>
    {/each}
  </div>

  <div class="pagination">
    <button class="pagination-btn" onclick={prev} disabled={offset === 0}>{'←'} PREV</button>
    <span class="pagination-info">PAGE {currentPage} OF {totalPages}</span>
    <button class="pagination-btn" onclick={next} disabled={offset + limit >= memoriesTotal}>NEXT {'→'}</button>
  </div>
{/if}

{#if editing}
  <div class="modal-backdrop" onclick={closeModal} role="presentation">
    <div class="modal" onclick={(e) => e.stopPropagation()} role="dialog" aria-label="Edit memory" tabindex="-1">
      <div class="modal-header">
        <span class="modal-label">EDIT MEMORY</span>
        <span class="modal-id mono">{editing.id} · v{editing.version}</span>
        <button
          class="modal-pin"
          class:modal-pin-active={editing.pinned > 0}
          onclick={togglePin}
          disabled={pinning || saving}
          title={editing.pinned > 0 ? 'Unpin (sujeita a decay)' : 'Pin (imune a decay)'}
        >{editing.pinned > 0 ? '★' : '☆'}</button>
        <button class="modal-close" onclick={closeModal} aria-label="Close">×</button>
      </div>

      <div class="modal-body">
        <label class="field">
          <span class="field-label">Title</span>
          <input class="modal-input" bind:value={editTitle} disabled={saving} />
        </label>

        <label class="field">
          <span class="field-label">Type</span>
          <select class="modal-input" bind:value={editType} disabled={saving}>
            {#each editableTypes as t}<option value={t}>{t}</option>{/each}
          </select>
        </label>

        <label class="field">
          <span class="field-label">Strength: {editStrength}/10</span>
          <input class="modal-range" type="range" min="1" max="10" bind:value={editStrength} disabled={saving} />
        </label>

        <label class="field">
          <span class="field-label">Content</span>
          <textarea class="modal-textarea" rows="8" bind:value={editContent} disabled={saving}></textarea>
        </label>

        <!-- Inline concept editor (B6) -->
        <div class="field">
          <span class="field-label">Concepts</span>
          <div class="concept-editor">
            {#each editConcepts as c}
              <span class="concept-chip">
                <ConceptTag label={c} />
                <button class="concept-remove" onclick={() => removeConcept(c)} disabled={saving} aria-label="Remove">×</button>
              </span>
            {/each}
            {#if editConcepts.length === 0}
              <span class="concept-empty">no concepts</span>
            {/if}
          </div>
          <form onsubmit={(e) => { e.preventDefault(); addConcept(); }} class="concept-add-form">
            <input
              class="modal-input concept-add-input"
              bind:value={newConcept}
              placeholder="Add concept and press Enter"
              disabled={saving}
            />
            <button type="submit" class="concept-add-btn" disabled={!newConcept.trim() || saving}>+</button>
          </form>
        </div>

        {#if modalError}
          <div class="modal-error">{modalError}</div>
        {/if}

        <!-- Why is this memory in my context? (B4) -->
        <div class="why-section">
          <button class="history-toggle" onclick={loadWhy} disabled={saving}>
            {whyExpanded ? '−' : '?'} Why was this retrieved? (score breakdown)
          </button>
          {#if whyExpanded}
            {#if whyLoading}
              <div class="history-loading">Searching…</div>
            {:else if whyResults.length === 0}
              <div class="history-empty">No matches for this title.</div>
            {:else}
              <ol class="why-list">
                {#each whyResults as r}
                  <li class="why-item" class:why-item-self={r.id === editing.id}>
                    <div class="why-head">
                      <span class="why-rank mono">#{r.rank}</span>
                      <span class="why-title">{r.title}</span>
                      {#if r.id === editing.id}<span class="why-self">this</span>{/if}
                    </div>
                    <div class="why-scores">
                      <span class="why-score-pair"><span class="why-score-key">Total</span><span class="mono">{r.score?.toFixed(3) ?? '—'}</span></span>
                      <span class="why-score-pair"><span class="why-score-key">BM25</span><span class="mono">{r.bm25Score?.toFixed(3) ?? '—'}</span></span>
                      <span class="why-score-pair"><span class="why-score-key">Vec</span><span class="mono">{r.vecScore?.toFixed(3) ?? '—'}</span></span>
                    </div>
                  </li>
                {/each}
              </ol>
            {/if}
          {/if}
        </div>

        {#if editing.version > 1}
          <div class="history-section">
            <button class="history-toggle" onclick={toggleHistory} disabled={saving}>
              {historyExpanded ? '−' : '+'} Version history (v1 … v{editing.version})
            </button>
            {#if historyExpanded}
              {#if historyLoading}
                <div class="history-loading">Loading…</div>
              {:else if history.length === 0}
                <div class="history-empty">No prior versions found.</div>
              {:else}
                <ol class="history-list">
                  {#each history as v}
                    <li class="history-item" class:history-item-current={v.id === editing.id}>
                      <div class="history-head">
                        <span class="history-version mono">v{v.version}</span>
                        <span class="history-id mono">{v.id}</span>
                        <span class="history-time">{fmtTime(v.updatedAt || v.createdAt)}</span>
                        {#if v.id === editing.id}<span class="history-current">current</span>{/if}
                      </div>
                      <div class="history-meta">
                        <span class="history-meta-item">{v.type}</span>
                        <span class="history-meta-item">strength {v.strength}/10</span>
                      </div>
                      <div class="history-title">{v.title}</div>
                      <p class="history-content">{v.content}</p>
                    </li>
                  {/each}
                </ol>
              {/if}
            {/if}
          </div>
        {/if}
      </div>

      <div class="modal-actions">
        {#if confirmingForget}
          <span class="modal-confirm-text">Forget this memory? This soft-deletes the latest version.</span>
          <button class="modal-btn modal-btn-danger" onclick={confirmForget} disabled={saving}>
            {saving ? 'Forgetting…' : 'Yes, forget'}
          </button>
          <button class="modal-btn" onclick={() => confirmingForget = false} disabled={saving}>Cancel</button>
        {:else}
          <button class="modal-btn modal-btn-danger-outline" onclick={() => confirmingForget = true} disabled={saving}>Forget</button>
          <div class="modal-actions-spacer"></div>
          <button class="modal-btn" onclick={closeModal} disabled={saving}>Cancel</button>
          <button class="modal-btn modal-btn-primary" onclick={saveEdit} disabled={saving}>
            {saving ? 'Saving…' : 'Save (new version)'}
          </button>
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  /* Pin star + memory-card-pinned: card com borda dourada se pinned. */
  .pin-star {
    color: var(--accent);
    font-size: 14px;
    margin-right: 4px;
    line-height: 1;
  }
  .memory-card-pinned {
    border-left: 2px solid var(--accent);
  }

  /* Pin button no modal header */
  .modal-pin {
    margin-left: auto;
    background: transparent;
    border: 1px solid var(--border);
    color: var(--text-muted);
    font-size: 16px;
    padding: 2px 10px;
    cursor: pointer;
    transition: all 0.15s var(--ease);
    line-height: 1.2;
  }
  .modal-pin:hover { color: var(--accent); border-color: var(--accent); }
  .modal-pin-active { color: var(--accent); border-color: var(--accent); background: var(--accent-muted); }
  .modal-pin:disabled { opacity: 0.5; cursor: not-allowed; }

  /* Inline concept editor */
  .concept-editor {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    align-items: center;
    padding: 8px 10px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    min-height: 38px;
  }
  .concept-chip { display: inline-flex; align-items: center; gap: 2px; }
  .concept-remove {
    background: transparent;
    border: none;
    color: var(--text-muted);
    font-size: 14px;
    cursor: pointer;
    padding: 0 4px;
    line-height: 1;
  }
  .concept-remove:hover { color: var(--danger, #ef4444); }
  .concept-empty {
    color: var(--text-muted);
    font-size: 11px;
    font-style: italic;
  }
  .concept-add-form {
    display: flex;
    gap: 6px;
    margin-top: 6px;
  }
  .concept-add-input { flex: 1; }
  .concept-add-btn {
    padding: 6px 14px;
    background: transparent;
    border: 1px solid var(--border);
    color: var(--text-muted);
    cursor: pointer;
    font-size: 14px;
  }
  .concept-add-btn:hover:not(:disabled) { color: var(--accent); border-color: var(--accent); }
  .concept-add-btn:disabled { opacity: 0.4; cursor: not-allowed; }

  /* Why section */
  .why-section { margin-top: 16px; padding-top: 14px; border-top: 1px solid var(--border); }
  .why-list { list-style: none; padding: 0; margin: 12px 0 0; display: flex; flex-direction: column; gap: 8px; }
  .why-item { padding: 10px 12px; background: var(--bg-secondary); border: 1px solid var(--border); }
  .why-item-self { border-color: var(--accent); }
  .why-head { display: flex; align-items: center; gap: 10px; margin-bottom: 4px; }
  .why-rank { font-size: 10px; color: var(--text-muted); }
  .why-title { font-size: 13px; font-weight: 600; color: var(--text-primary); flex: 1; }
  .why-self { font-size: 9px; padding: 1px 6px; background: var(--accent); color: #030303; font-weight: 700; letter-spacing: 0.08em; text-transform: uppercase; }
  .why-scores { display: flex; gap: 14px; font-size: 11px; }
  .why-score-pair { display: inline-flex; gap: 4px; align-items: baseline; }
  .why-score-key { color: var(--text-muted); font-size: 9px; text-transform: uppercase; letter-spacing: 0.08em; }

  /* Time-travel slider — banda fina acima dos filtros, só aparece como
     mudança de data quando arrastada. Na posição 0 vira o "now" badge. */
  .time-travel-bar {
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 10px 14px;
    margin-bottom: 14px;
    background: var(--bg-card);
    border: 1px solid var(--border);
  }
  .tt-label {
    font-family: var(--font-ui);
    font-size: 9px;
    font-weight: 700;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.12em;
    flex-shrink: 0;
  }
  .tt-value {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--accent);
    font-weight: 600;
    min-width: 80px;
  }
  .tt-slider {
    flex: 1;
    accent-color: var(--accent);
    height: 4px;
  }
  .tt-reset {
    background: transparent;
    border: 1px solid var(--border);
    color: var(--text-muted);
    font-family: var(--font-ui);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    padding: 4px 10px;
    cursor: pointer;
    flex-shrink: 0;
  }
  .tt-reset:hover {
    color: var(--accent);
    border-color: var(--accent);
  }

  /* Filter bar */
  .filter-bar {
    display: flex;
    align-items: center;
    gap: 0;
    margin-bottom: 24px;
    border-bottom: 1px solid var(--border);
  }
  .filter-btn {
    background: none;
    border: none;
    border-bottom: 2px solid transparent;
    padding: 10px 16px;
    font-family: var(--font-ui);
    font-size: 11px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.1em;
    color: var(--text-muted);
    cursor: pointer;
    transition: color 0.2s var(--ease), border-color 0.2s var(--ease);
  }
  .filter-btn:hover { color: var(--text-primary); }
  .filter-btn.active {
    color: var(--accent);
    border-bottom-color: var(--accent);
  }

  /* Memory list */
  .memory-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  /* Memory card — now a button */
  .memory-card {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-left: 2px solid var(--accent);
    padding: 20px 24px;
    transition: border-color 0.3s var(--ease), box-shadow 0.3s var(--ease);
    text-align: left;
    width: 100%;
    cursor: pointer;
    color: inherit;
    font: inherit;
  }
  .memory-card:hover {
    border-color: var(--accent);
    box-shadow: var(--shadow-hover);
  }

  .card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 10px;
  }
  .card-header-left {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .memory-title {
    font-family: var(--font-display);
    font-size: 16px;
    font-weight: 700;
    color: var(--text-primary);
    letter-spacing: -0.02em;
    line-height: 1.3;
  }
  .version-label {
    font-size: 11px;
    color: var(--text-muted);
    flex-shrink: 0;
  }

  .memory-content {
    font-family: var(--font-body);
    font-size: 13px;
    color: var(--text-dim);
    line-height: 1.6;
    margin-bottom: 14px;
  }

  .memory-meta {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .strength-row {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .strength-label {
    font-size: 10px;
    font-family: var(--font-ui);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.1em;
    color: var(--text-muted);
    flex-shrink: 0;
  }
  .strength-val {
    font-size: 11px;
    color: var(--text-muted);
    flex-shrink: 0;
  }

  .concepts {
    display: flex;
    gap: 4px;
    flex-wrap: wrap;
  }

  /* Loading skeleton */
  .loading-state { display: flex; flex-direction: column; gap: 8px; }
  .skeleton-card {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-left: 2px solid var(--border-hover);
    padding: 20px 24px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .skeleton-line {
    height: 12px;
    width: 60%;
    background: var(--bg-hover);
    animation: pulse 1.5s ease-in-out infinite;
  }
  .skeleton-line.wide { width: 80%; height: 16px; }
  .skeleton-line.narrow { width: 40%; }
  @keyframes pulse {
    0%, 100% { opacity: 0.3; }
    50% { opacity: 0.7; }
  }

  /* Modal */
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    z-index: 1000;
    display: flex;
    justify-content: center;
    align-items: flex-start;
    padding-top: 60px;
  }
  .modal {
    width: min(720px, calc(100vw - 32px));
    max-height: calc(100vh - 100px);
    display: flex;
    flex-direction: column;
    background: var(--bg-secondary);
    border: 1px solid var(--accent);
    border-top-width: 2px;
  }
  .modal-header {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 14px 18px;
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }
  .modal-label {
    font-family: var(--font-ui);
    font-size: 10px;
    font-weight: 700;
    color: var(--accent);
    text-transform: uppercase;
    letter-spacing: 0.12em;
  }
  .modal-id {
    flex: 1;
    font-size: 11px;
    color: var(--text-muted);
  }
  .modal-close {
    background: transparent;
    border: none;
    font-size: 22px;
    color: var(--text-muted);
    cursor: pointer;
    padding: 0 4px;
  }
  .modal-close:hover { color: var(--text-primary); }
  .modal-body {
    padding: 18px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .field {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .field-label {
    font-family: var(--font-ui);
    font-size: 10px;
    font-weight: 700;
    color: var(--text-dim);
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }

  .modal-input {
    padding: 10px 12px;
    background: var(--bg-card);
    border: 1px solid var(--border);
    color: var(--text-primary);
    font-family: var(--font-ui);
    font-size: 14px;
  }
  .modal-input:focus { outline: none; border-color: var(--accent); }
  .modal-input:disabled { opacity: 0.6; }

  .modal-textarea {
    padding: 10px 12px;
    background: var(--bg-card);
    border: 1px solid var(--border);
    color: var(--text-primary);
    font-family: var(--font-body);
    font-size: 14px;
    line-height: 1.5;
    resize: vertical;
  }
  .modal-textarea:focus { outline: none; border-color: var(--accent); }

  .modal-range {
    width: 100%;
    accent-color: var(--accent);
    height: 4px;
  }

  .modal-error {
    padding: 10px 12px;
    background: rgba(239, 68, 68, 0.06);
    border: 1px solid rgba(239, 68, 68, 0.25);
    color: var(--danger);
    font-size: 13px;
  }

  .modal-actions {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 14px 18px;
    border-top: 1px solid var(--border);
    flex-shrink: 0;
  }
  .modal-actions-spacer { flex: 1; }
  .modal-btn {
    padding: 8px 16px;
    background: transparent;
    border: 1px solid var(--border);
    color: var(--text-secondary);
    font-family: var(--font-ui);
    font-size: 12px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    cursor: pointer;
    transition: all 0.15s var(--ease);
  }
  .modal-btn:hover:not(:disabled) {
    border-color: var(--accent);
    color: var(--accent);
  }
  .modal-btn:disabled { opacity: 0.4; cursor: not-allowed; }
  .modal-btn-primary {
    background: var(--accent);
    color: #030303;
    border-color: var(--accent);
  }
  .modal-btn-primary:hover:not(:disabled) {
    background: var(--accent-hover);
    border-color: var(--accent-hover);
    color: #030303;
  }
  .modal-btn-danger {
    background: var(--danger, #ef4444);
    color: #fff;
    border-color: var(--danger, #ef4444);
  }
  .modal-btn-danger:hover:not(:disabled) {
    opacity: 0.9;
    color: #fff;
  }
  .modal-btn-danger-outline {
    border-color: rgba(239, 68, 68, 0.4);
    color: var(--danger, #ef4444);
  }
  .modal-btn-danger-outline:hover:not(:disabled) {
    background: rgba(239, 68, 68, 0.08);
    border-color: var(--danger, #ef4444);
    color: var(--danger, #ef4444);
  }
  .modal-confirm-text {
    flex: 1;
    font-size: 13px;
    color: var(--text-primary);
  }

  /* History panel */
  .history-section {
    margin-top: 12px;
    padding-top: 12px;
    border-top: 1px solid var(--border);
  }
  .history-toggle {
    background: transparent;
    border: none;
    color: var(--text-muted);
    font-family: var(--font-ui);
    font-size: 11px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    cursor: pointer;
    padding: 4px 0;
  }
  .history-toggle:hover { color: var(--accent); }
  .history-toggle:disabled { opacity: 0.5; cursor: not-allowed; }
  .history-loading {
    padding: 12px 0;
    font-family: var(--font-ui);
    font-size: 11px;
    color: var(--text-muted);
  }
  .history-empty {
    padding: 12px 0;
    font-size: 12px;
    color: var(--text-muted);
  }
  .history-list {
    list-style: none;
    margin: 12px 0 0 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 8px;
    max-height: 300px;
    overflow-y: auto;
  }
  .history-item {
    padding: 10px 12px;
    border: 1px solid var(--border);
    background: var(--bg-card);
  }
  .history-item-current {
    border-color: var(--accent);
    background: var(--accent-muted);
  }
  .history-head {
    display: flex;
    align-items: baseline;
    gap: 10px;
    margin-bottom: 4px;
  }
  .history-version {
    font-size: 12px;
    color: var(--accent);
    font-weight: 700;
  }
  .history-id {
    font-size: 10px;
    color: var(--text-muted);
  }
  .history-time {
    font-size: 11px;
    color: var(--text-muted);
    flex: 1;
  }
  .history-current {
    font-family: var(--font-ui);
    font-size: 9px;
    font-weight: 700;
    text-transform: uppercase;
    color: var(--accent);
    letter-spacing: 0.1em;
  }
  .history-meta {
    display: flex;
    gap: 12px;
    margin-bottom: 4px;
  }
  .history-meta-item {
    font-family: var(--font-ui);
    font-size: 10px;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }
  .history-title {
    font-size: 13px;
    font-weight: 600;
    color: var(--text-primary);
    margin-bottom: 4px;
  }
  .history-content {
    font-size: 12px;
    color: var(--text-dim);
    line-height: 1.5;
    white-space: pre-wrap;
    margin: 0;
  }
</style>
