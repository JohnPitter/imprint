<script lang="ts">
  import { api } from '../../lib/api';

  type Source = { id: string; sessionId: string; type: string; title: string; score: number };

  let query = $state('');
  let answer = $state('');
  let sources: Source[] = $state([]);
  let skipped = $state('');
  let loading = $state(false);
  let lastQuery = $state('');
  let error = $state('');

  async function ask(e?: Event) {
    e?.preventDefault();
    const q = query.trim();
    if (!q || loading) return;
    loading = true;
    error = '';
    answer = '';
    sources = [];
    skipped = '';
    lastQuery = q;
    try {
      const r = await api.recall(q, 8) as any;
      answer = r.answer || '';
      sources = r.sources || [];
      skipped = r.skipped || '';
    } catch (err: any) {
      error = err?.message || 'Recall failed';
    }
    loading = false;
  }

  // Cite [1] [2] etc. → highlight when hovering the corresponding source.
  // Keep it simple: render the answer as plain text; the citation numbers are
  // already conventional reading.

  function typeBadge(t: string): string {
    const k = (t || '').toLowerCase();
    if (k === 'memory') return 'badge-purple';
    if (k === 'lesson') return 'badge-success';
    if (k === 'insight') return 'badge-warning';
    if (k === 'observation') return 'badge-info';
    return 'badge-accent';
  }
</script>

<div class="recall-container">
  <div class="recall-header">
    <div class="gold-line"></div>
    <h3>Recall</h3>
    <p class="recall-hint">Ask a question — the LLM synthesises an answer from your memories and observations, and cites the sources.</p>
  </div>

  <form class="recall-form" onsubmit={ask}>
    <input
      class="input recall-input"
      bind:value={query}
      placeholder="What did I learn about JWT this week?"
      disabled={loading}
    />
    <button class="recall-submit" type="submit" disabled={loading || !query.trim()}>
      {loading ? 'Thinking…' : 'Ask'}
    </button>
  </form>

  {#if error}
    <div class="recall-error">{error}</div>
  {/if}

  {#if loading && !answer}
    <div class="recall-skeleton">
      <div class="skeleton-line wide"></div>
      <div class="skeleton-line"></div>
      <div class="skeleton-line narrow"></div>
    </div>
  {/if}

  {#if answer || sources.length > 0}
    <div class="recall-result">
      <div class="recall-question">
        <span class="recall-question-label">Q</span>
        <span class="recall-question-text">{lastQuery}</span>
      </div>

      {#if answer}
        <div class="recall-answer">{answer}</div>
      {/if}

      {#if skipped}
        <div class="recall-skipped">{skipped}</div>
      {/if}

      {#if sources.length > 0}
        <div class="recall-sources">
          <div class="recall-sources-header">
            <div class="gold-line gold-line-tight"></div>
            <span class="recall-sources-label">SOURCES</span>
            <span class="recall-sources-count">{sources.length}</span>
          </div>
          <ol class="recall-source-list">
            {#each sources as src, i}
              <li class="recall-source">
                <span class="recall-source-num mono">[{i + 1}]</span>
                <span class="badge {typeBadge(src.type)}">{src.type || 'note'}</span>
                <span class="recall-source-title">{src.title || '(untitled)'}</span>
                <span class="recall-source-score mono">{(src.score * 1000).toFixed(0)}</span>
              </li>
            {/each}
          </ol>
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .recall-container { display: flex; flex-direction: column; max-width: 880px; }

  .recall-header {
    margin-bottom: 24px;
  }
  .recall-header h3 {
    font-family: var(--font-display);
    font-size: 18px;
    font-weight: 700;
    letter-spacing: -0.03em;
    margin-top: 8px;
  }
  .recall-hint {
    font-size: 13px;
    color: var(--text-muted);
    margin-top: 8px;
    line-height: 1.5;
  }

  .recall-form {
    display: flex;
    gap: 12px;
    margin-bottom: 24px;
  }
  .recall-input {
    flex: 1;
    padding: 12px 16px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    color: var(--text-primary);
    font-family: var(--font-ui);
    font-size: 14px;
    transition: border-color 0.2s var(--ease);
  }
  .recall-input:focus {
    outline: none;
    border-color: var(--accent);
    box-shadow: 0 0 0 1px var(--accent-muted);
  }
  .recall-input::placeholder {
    color: var(--text-muted);
  }
  .recall-input:disabled { opacity: 0.6; cursor: not-allowed; }

  .recall-submit {
    padding: 12px 24px;
    background: var(--accent);
    color: #030303;
    border: 1px solid var(--accent);
    font-family: var(--font-ui);
    font-size: 12px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    cursor: pointer;
    transition: all 0.2s var(--ease);
  }
  .recall-submit:hover:not(:disabled) { background: var(--accent-hover); border-color: var(--accent-hover); }
  .recall-submit:disabled { opacity: 0.4; cursor: not-allowed; }

  .recall-error {
    padding: 12px 16px;
    margin-bottom: 16px;
    background: rgba(239, 68, 68, 0.06);
    border: 1px solid rgba(239, 68, 68, 0.2);
    color: var(--danger);
    font-family: var(--font-ui);
    font-size: 13px;
  }

  .recall-skeleton {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 18px 20px;
    background: var(--bg-card);
    border: 1px solid var(--border);
  }
  .skeleton-line {
    height: 12px;
    width: 80%;
    background: var(--bg-hover);
    animation: pulse 1.4s ease-in-out infinite;
  }
  .skeleton-line.wide { width: 100%; }
  .skeleton-line.narrow { width: 50%; }
  @keyframes pulse {
    0%, 100% { opacity: 0.3; }
    50% { opacity: 0.7; }
  }

  .recall-result {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-top: 2px solid var(--accent);
    padding: 24px;
  }
  .recall-question {
    display: flex;
    align-items: baseline;
    gap: 12px;
    margin-bottom: 18px;
    padding-bottom: 14px;
    border-bottom: 1px solid var(--border);
  }
  .recall-question-label {
    font-family: var(--font-display);
    font-size: 18px;
    font-weight: 700;
    color: var(--accent);
    flex-shrink: 0;
  }
  .recall-question-text {
    font-size: 16px;
    font-weight: 600;
    color: var(--text-primary);
    line-height: 1.4;
  }
  .recall-answer {
    font-family: var(--font-body);
    font-size: 15px;
    line-height: 1.65;
    color: var(--text-primary);
    white-space: pre-wrap;
  }
  .recall-skipped {
    margin-top: 14px;
    padding: 10px 14px;
    background: rgba(234, 179, 8, 0.06);
    border-left: 2px solid var(--warning, #eab308);
    color: var(--text-muted);
    font-family: var(--font-ui);
    font-size: 12px;
  }

  .recall-sources {
    margin-top: 24px;
    padding-top: 20px;
    border-top: 1px solid var(--border);
  }
  .recall-sources-header {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 14px;
  }
  .gold-line-tight { width: 24px; margin-bottom: 0; height: 2px; background: var(--accent); }
  .recall-sources-label {
    font-family: var(--font-ui);
    font-size: 10px;
    font-weight: 700;
    color: var(--accent);
    text-transform: uppercase;
    letter-spacing: 0.12em;
  }
  .recall-sources-count {
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--text-muted);
  }

  .recall-source-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .recall-source {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 12px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    transition: border-color 0.15s var(--ease);
  }
  .recall-source:hover { border-color: var(--accent); }
  .recall-source-num {
    font-size: 11px;
    color: var(--accent);
    font-weight: 700;
    flex-shrink: 0;
    width: 28px;
  }
  .recall-source-title {
    flex: 1;
    font-size: 13px;
    color: var(--text-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .recall-source-score {
    font-size: 10px;
    color: var(--text-muted);
    flex-shrink: 0;
  }
</style>
