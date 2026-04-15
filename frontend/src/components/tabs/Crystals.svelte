<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../../lib/api';
  import { timeAgo } from '../../lib/format';

  let crystals: any[] = [];
  let loading = true;

  onMount(async () => {
    try {
      const r = await api.listCrystals(50);
      crystals = r.crystals || [];
    } catch(e) { console.error(e); }
    loading = false;
  });
</script>

<div class="crystals-container">
  <div class="crystals-header">
    <div>
      <div class="gold-line"></div>
      <h3>Crystals</h3>
    </div>
    <span class="crystal-count">{crystals.length} CRYSTALS</span>
  </div>

  {#if loading}
    <div class="loading-cards">
      {#each Array(3) as _}
        <div class="skeleton-card"></div>
      {/each}
    </div>
  {:else if crystals.length === 0}
    <div class="empty-state">
      <div class="icon" style="font-size:32px; opacity:0.15">&#9670;</div>
      <p>No crystals yet. Crystals are generated from completed action chains.</p>
    </div>
  {:else}
    <div class="crystal-list">
      {#each crystals as c}
        <div class="crystal-card">
          <div class="crystal-gold-border"></div>
          <div class="crystal-body">
            <p class="crystal-narrative">{c.narrative}</p>

            {#if c.keyOutcomes?.length > 0}
              <div class="crystal-section">
                <span class="crystal-section-label">KEY OUTCOMES</span>
                <ul class="outcomes-list">
                  {#each c.keyOutcomes as o}
                    <li>{o}</li>
                  {/each}
                </ul>
              </div>
            {/if}

            {#if c.lessonsLearned?.length > 0}
              <div class="crystal-section">
                <span class="crystal-section-label">LESSONS</span>
                <ul class="outcomes-list">
                  {#each c.lessonsLearned as l}
                    <li>{l}</li>
                  {/each}
                </ul>
              </div>
            {/if}

            {#if c.filesAffected?.length > 0}
              <div class="crystal-files">
                {#each c.filesAffected as f}
                  <span class="file-badge">{f}</span>
                {/each}
              </div>
            {/if}

            <div class="crystal-footer">
              <span class="mono crystal-time">{timeAgo(c.createdAt)}</span>
            </div>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .crystals-container { display: flex; flex-direction: column; }
  .crystals-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    margin-bottom: 24px;
  }
  .crystals-header h3 {
    font-family: var(--font-display);
    font-size: 16px;
    font-weight: 600;
    letter-spacing: -0.02em;
  }
  .crystal-count {
    font-family: var(--font-ui);
    font-size: 9px;
    font-weight: 700;
    color: var(--text-muted);
    letter-spacing: 0.1em;
    margin-top: 4px;
  }

  .crystal-list { display: flex; flex-direction: column; gap: 24px; }

  .crystal-card {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: 0;
    position: relative;
    transition: border-color 0.3s var(--ease), box-shadow 0.3s var(--ease);
  }
  .crystal-card:hover {
    border-color: var(--accent);
    box-shadow: var(--shadow-hover);
  }
  .crystal-gold-border {
    height: 2px;
    background: var(--accent);
    width: 100%;
  }
  .crystal-body { padding: 28px 32px; }

  .crystal-narrative {
    font-family: var(--font-body);
    font-size: 15px;
    line-height: 1.7;
    color: var(--text-secondary);
  }

  .crystal-section { margin-top: 20px; }
  .crystal-section-label {
    display: block;
    font-family: var(--font-ui);
    font-size: 10px;
    font-weight: 700;
    color: var(--accent);
    text-transform: uppercase;
    letter-spacing: 0.12em;
    margin-bottom: 10px;
  }
  .outcomes-list {
    list-style: none;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .outcomes-list li {
    font-family: var(--font-body);
    font-size: 13px;
    line-height: 1.6;
    color: var(--text-dim);
    padding-left: 16px;
    position: relative;
  }
  .outcomes-list li::before {
    content: '\25AA';
    position: absolute;
    left: 0;
    color: var(--accent);
    font-size: 8px;
    top: 3px;
  }

  .crystal-files {
    display: flex;
    gap: 6px;
    flex-wrap: wrap;
    margin-top: 20px;
    padding-top: 16px;
    border-top: 1px solid var(--border);
  }
  .file-badge {
    display: inline-flex;
    align-items: center;
    padding: 3px 10px;
    border: 1px solid var(--border-hover);
    background: transparent;
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--text-dim);
    letter-spacing: 0;
    border-radius: 0;
    transition: border-color 0.2s var(--ease), color 0.2s var(--ease);
  }
  .file-badge:hover {
    border-color: var(--accent);
    color: var(--accent);
  }

  .crystal-footer {
    display: flex;
    justify-content: flex-end;
    margin-top: 16px;
  }
  .crystal-time {
    font-size: 11px;
    color: var(--text-muted);
  }

  /* Loading */
  .loading-cards { display: flex; flex-direction: column; gap: 24px; }
  .skeleton-card {
    height: 180px;
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-top: 2px solid var(--accent);
    animation: pulse 1.5s ease-in-out infinite;
  }
  @keyframes pulse {
    0%, 100% { opacity: 0.4; }
    50% { opacity: 0.8; }
  }
</style>
