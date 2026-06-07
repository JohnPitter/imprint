<script lang="ts">
  // Rooted layer (intuition) inspection screen — invariant 11: every intuition
  // is always visible with its evidence, force and last contradiction, and can
  // be demoted or deleted by hand even though birth is automatic. Also hosts the
  // A5 memory-governance controls (export / purge / reset).
  import { onMount } from 'svelte';
  import { api } from '../../lib/api';

  interface Intuition {
    id: string;
    project: string;
    statement: string;
    strength: number;
    evidenceIds: string[];
    evidenceCount: number;
    concepts: string[];
    lastContradictedAt?: string;
    contradictionCount: number;
    status: string;
    createdAt: string;
  }
  interface Contradiction { id: string; ts: string; memoryId: string; detail: string; strengthDelta: number; }

  let items: Intuition[] = $state([]);
  let loading = $state(true);
  let error = $state('');
  let project = $state('');
  let expanded: string | null = $state(null);
  let contradictions: Contradiction[] = $state([]);
  let notice = $state('');

  async function load() {
    loading = true;
    error = '';
    try {
      const r: any = await api.listIntuitions(project);
      items = r.intuitions || [];
    } catch (e: any) {
      error = e?.message || 'Failed to load intuitions';
    } finally {
      loading = false;
    }
  }
  onMount(load);

  async function toggle(id: string) {
    if (expanded === id) { expanded = null; return; }
    expanded = id;
    contradictions = [];
    try {
      const r: any = await api.intuitionContradictions(id);
      contradictions = r.contradictions || [];
    } catch { /* ignore */ }
  }

  async function demote(id: string) {
    await api.demoteIntuition(id);
    await load();
  }
  async function del(id: string) {
    if (!confirm('Delete this intuition permanently?')) return;
    await api.deleteIntuition(id);
    await load();
  }
  async function detect() {
    if (!project) { notice = 'Enter a project to run detection.'; return; }
    notice = 'Running convergence detection…';
    try {
      const r: any = await api.detectIntuitions(project);
      notice = `Detection done — ${r.count} new intuition(s) rooted.`;
      await load();
    } catch (e: any) {
      notice = 'Detection failed: ' + (e?.message || '');
    }
  }

  // ── A5 governance ──
  async function exportMem() {
    if (!project) { notice = 'Enter a project to export.'; return; }
    try {
      const data = await api.exportMemory(project);
      const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url; a.download = `imprint-memory-${project}.json`; a.click();
      URL.revokeObjectURL(url);
    } catch (e: any) { notice = 'Export failed: ' + (e?.message || ''); }
  }
  async function purge() {
    if (!project) { notice = 'Enter a project to purge.'; return; }
    if (!confirm(`Permanently delete ALL memory for "${project}"? This cannot be undone.`)) return;
    try {
      const r: any = await api.purgeMemory(project);
      notice = `Purged "${project}": ` + JSON.stringify(r.deleted);
      await load();
    } catch (e: any) { notice = 'Purge failed: ' + (e?.message || ''); }
  }
  async function reset() {
    if (!confirm('Reset ALL memory across every repo back to cold start? This cannot be undone.')) return;
    if (!confirm('Are you absolutely sure? Everything will be erased.')) return;
    try {
      await api.resetMemory();
      notice = 'All memory reset to cold start.';
      await load();
    } catch (e: any) { notice = 'Reset failed: ' + (e?.message || ''); }
  }

  function forceClass(s: number): string { return s >= 6 ? 'strong' : s >= 4 ? 'mid' : 'weak'; }
</script>

<div class="intu">
  <div class="head">
    <h2>Intuitions <span class="sub">rooted layer · always inspectable</span></h2>
    <div class="controls">
      <input class="proj" placeholder="project" bind:value={project} onkeydown={(e) => e.key === 'Enter' && load()} />
      <button onclick={load}>Filter</button>
      <button onclick={detect}>Detect now</button>
    </div>
  </div>

  <p class="explain">
    Intuitions are cross-cutting reasoning premises that form <strong>automatically</strong> when many
    refined insights converge. They sit resident at max priority and <strong>auto-weaken</strong> when
    contradicted. You can't create one by hand, but you can always inspect its evidence and demote or
    delete it. A specific refined memory always overrides a general intuition.
  </p>

  {#if notice}<div class="notice">{notice}</div>{/if}

  {#if loading}
    <div class="msg">Loading…</div>
  {:else if error}
    <div class="msg err">{error}</div>
  {:else if items.length === 0}
    <div class="msg">No intuitions yet. They emerge only after enough refined insights converge — cold start is empty by design.</div>
  {:else}
    <ul class="list">
      {#each items as it}
        <li class="card" class:demoted={it.status !== 'active'}>
          <div class="card-top">
            <span class="force {forceClass(it.strength)}">{it.strength.toFixed(0)}</span>
            <span class="statement">{it.statement}</span>
            <span class="status status-{it.status}">{it.status}</span>
          </div>
          <div class="meta">
            <span>project: {it.project || '—'}</span>
            <span>evidence: {it.evidenceCount}</span>
            <span>contradictions: {it.contradictionCount}</span>
            {#if it.lastContradictedAt}<span>last contra: {it.lastContradictedAt}</span>{/if}
            {#if it.concepts?.length}<span>concepts: {it.concepts.join(', ')}</span>{/if}
          </div>
          <div class="actions">
            <button onclick={() => toggle(it.id)}>{expanded === it.id ? 'Hide' : 'Evidence & contradictions'}</button>
            {#if it.status === 'active'}<button onclick={() => demote(it.id)}>Demote</button>{/if}
            <button class="danger" onclick={() => del(it.id)}>Delete</button>
          </div>
          {#if expanded === it.id}
            <div class="detail">
              <div class="detail-section">
                <strong>Evidence (source memories):</strong>
                <code>{it.evidenceIds.join(', ') || '—'}</code>
              </div>
              <div class="detail-section">
                <strong>Contradiction log:</strong>
                {#if contradictions.length === 0}
                  <span class="dim"> none</span>
                {:else}
                  <ul class="contra">
                    {#each contradictions as c}
                      <li><span class="dim">{c.ts}</span> −{c.strengthDelta} · mem {c.memoryId}: {c.detail}</li>
                    {/each}
                  </ul>
                {/if}
              </div>
            </div>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}

  <div class="danger-zone">
    <h3>Memory governance</h3>
    <p class="dim">Your memory is yours — export it, purge a repo, or reset everything.</p>
    <div class="dz-actions">
      <button onclick={exportMem}>Export "{project || '…'}"</button>
      <button class="danger" onclick={purge}>Purge "{project || '…'}"</button>
      <button class="danger" onclick={reset}>Reset everything</button>
    </div>
  </div>
</div>

<style>
  .intu { max-width: 920px; }
  .head { display:flex; align-items:center; justify-content:space-between; gap:16px; margin-bottom:12px; flex-wrap:wrap; }
  .head h2 { font-size:18px; font-weight:700; color:var(--text-primary); margin:0; }
  .sub { font-size:11px; color:var(--text-muted); font-weight:400; margin-left:8px; }
  .controls { display:flex; gap:6px; }
  .proj { background:var(--bg-secondary); border:1px solid var(--border); color:var(--text-primary); padding:6px 10px; font-family:var(--font-mono); font-size:12px; }
  .controls button, .actions button, .dz-actions button {
    padding:6px 12px; background:transparent; border:1px solid var(--border); color:var(--text-secondary);
    font-family:var(--font-ui); font-size:11px; cursor:pointer;
  }
  .controls button:hover, .actions button:hover { color:var(--accent); border-color:var(--accent); }
  .explain { font-size:12px; line-height:1.7; color:var(--text-dim); background:var(--bg-secondary); border:1px solid var(--border); padding:12px 16px; margin-bottom:16px; }
  .explain strong { color:var(--text-secondary); }
  .notice { font-size:12px; color:var(--accent); padding:8px 12px; border:1px solid var(--accent); margin-bottom:12px; }
  .msg { padding:24px; text-align:center; font-family:var(--font-mono); font-size:12px; color:var(--text-muted); }
  .msg.err { color:var(--danger, #ef4444); }

  .list { list-style:none; margin:0; padding:0; }
  .card { border:1px solid var(--border); background:var(--bg-secondary); padding:14px; margin-bottom:10px; }
  .card.demoted { opacity:0.6; }
  .card-top { display:flex; align-items:center; gap:10px; }
  .force { font-family:var(--font-mono); font-weight:700; font-size:14px; width:28px; height:28px; display:flex; align-items:center; justify-content:center; border:1px solid var(--border); flex-shrink:0; }
  .force.strong { color:var(--success, #10b981); border-color:var(--success, #10b981); }
  .force.mid { color:var(--accent); border-color:var(--accent); }
  .force.weak { color:var(--danger, #ef4444); border-color:var(--danger, #ef4444); }
  .statement { flex:1; font-size:14px; color:var(--text-primary); font-weight:500; }
  .status { font-family:var(--font-mono); font-size:9px; text-transform:uppercase; letter-spacing:0.1em; padding:2px 6px; border:1px solid var(--border); }
  .status-active { color:var(--success, #10b981); border-color:var(--success, #10b981); }
  .status-demoted { color:var(--text-muted); }
  .meta { display:flex; flex-wrap:wrap; gap:12px; margin-top:8px; font-size:11px; color:var(--text-muted); font-family:var(--font-mono); }
  .actions { display:flex; gap:6px; margin-top:10px; }
  .danger { color:var(--danger, #ef4444) !important; border-color:var(--danger, #ef4444) !important; }
  .detail { margin-top:12px; padding-top:12px; border-top:1px solid var(--border); font-size:12px; }
  .detail-section { margin-bottom:8px; color:var(--text-dim); }
  .detail-section code { color:var(--text-muted); font-size:11px; }
  .contra { margin:6px 0 0 0; padding-left:16px; }
  .contra li { font-size:11px; color:var(--text-dim); margin-bottom:3px; }
  .dim { color:var(--text-muted); }

  .danger-zone { margin-top:28px; border:1px solid var(--danger, #ef4444); border-opacity:0.4; padding:16px; }
  .danger-zone h3 { font-size:13px; color:var(--danger, #ef4444); margin:0 0 4px 0; text-transform:uppercase; letter-spacing:0.06em; }
  .dz-actions { display:flex; gap:8px; margin-top:12px; flex-wrap:wrap; }
</style>
