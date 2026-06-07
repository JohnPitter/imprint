<script lang="ts">
  // Phase 1 — token economy meter. Shows the saldo (context saved − Haiku
  // spent) honestly: it can be negative during cold start, and we always show
  // the baseline ("proxy") next to the number rather than a bare percentage.
  import { onMount } from 'svelte';
  import { api } from '../../lib/api';
  import Sparkline from '../Sparkline.svelte';

  interface DayPoint { day: string; haikuTokens: number; savedTokens: number; saldo: number; }
  interface Budget {
    perSessionLimit: number;
    perDayLimit: number;
    daySpent: number;
    day: string;
    pausedSessions: number;
    dayExceeded: boolean;
  }
  interface Economy {
    project: string;
    sinceDays: number;
    haikuInputTokens: number;
    haikuOutputTokens: number;
    haikuTokens: number;
    savedTokens: number;
    injectedTokens: number;
    saldoTokens: number;
    spendCalls: number;
    injectionItems: number;
    savingEvents: number;
    usedRatio: number;
    confidence: string;
    daily: DayPoint[];
    plan: string;
    saldoMoedaUSD: number;
    haikuCostUSD: number;
    folegoPct: number;
    budget: Budget;
  }

  let data: Economy | null = $state(null);
  let loading = $state(true);
  let error = $state('');
  let sinceDays = $state(0); // 0 = all time

  async function load() {
    loading = true;
    error = '';
    try {
      data = (await api.economy('', sinceDays)) as Economy;
    } catch (e: any) {
      error = e?.message || 'Failed to load economy';
    } finally {
      loading = false;
    }
  }

  onMount(load);

  function setWindow(d: number) {
    sinceDays = d;
    load();
  }

  // Signed, abbreviated token count: -1.5K, +12.3K, 0.
  function fmtSigned(n: number): string {
    const sign = n > 0 ? '+' : n < 0 ? '−' : '';
    const a = Math.abs(n);
    if (a >= 1_000_000) return `${sign}${(a / 1_000_000).toFixed(1)}M`;
    if (a >= 1_000) return `${sign}${(a / 1_000).toFixed(1)}K`;
    return `${sign}${a}`;
  }
  function fmt(n: number): string {
    if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
    if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
    return String(n);
  }

  function fmtUSD(n: number): string {
    const sign = n > 0 ? '+' : n < 0 ? '−' : '';
    return `${sign}$${Math.abs(n).toFixed(2)}`;
  }

  let saldo = $derived(data?.saldoTokens ?? 0);
  let coldStart = $derived(!!data && data.spendCalls === 0 && data.injectionItems === 0);
  let usedPct = $derived(data ? Math.round((data.usedRatio || 0) * 100) : 0);
  let saldoSeries = $derived((data?.daily ?? []).map((d) => d.saldo));
  let saldoClass = $derived(saldo > 0 ? 'pos' : saldo < 0 ? 'neg' : 'zero');
  let isAPI = $derived(data?.plan === 'api');
  // Top number adapts to the plan: currency for API, breathing room for Pro/Max.
  let heroValue = $derived(
    !data ? '' : isAPI ? fmtUSD(data.saldoMoedaUSD) : fmtSigned(saldo)
  );
  let heroLabel = $derived(isAPI ? 'SALDO EM MOEDA' : 'SALDO DE TOKEN (FÔLEGO)');
  let heroSub = $derived(
    isAPI
      ? 'valor poupado − custo Haiku · USD'
      : `contexto poupado − Haiku gasto · tokens · ~${(data?.folegoPct ?? 0).toFixed(1)}% da janela`
  );
  // Budget meter: how much of the daily Haiku ceiling is used (0 = unlimited).
  let budgetPct = $derived(
    data && data.budget.perDayLimit > 0
      ? Math.min(100, Math.round((data.budget.daySpent / data.budget.perDayLimit) * 100))
      : 0
  );
</script>

<div class="economy">
  <div class="head">
    <h2>Token Economy</h2>
    <div class="windows">
      <button class:active={sinceDays === 0} onclick={() => setWindow(0)}>All time</button>
      <button class:active={sinceDays === 7} onclick={() => setWindow(7)}>7d</button>
      <button class:active={sinceDays === 30} onclick={() => setWindow(30)}>30d</button>
    </div>
  </div>

  {#if loading}
    <div class="msg">Loading…</div>
  {:else if error}
    <div class="msg err">{error}</div>
  {:else if data}
    {#if coldStart}
      <div class="cold">
        <strong>Building memory.</strong> No Haiku spent and nothing injected yet in this window.
        The saldo turns meaningful once memories are reused — it may sit near zero or slightly
        negative while memory is being built. That investment period is shown honestly, not hidden.
      </div>
    {/if}

    <!-- Top number: the saldo, adapted to the plan. Honest sign + color. -->
    <div class="hero">
      <div class="hero-label">{heroLabel} <span class="conf">{data.plan || '—'} · {data.confidence}</span></div>
      <div class="hero-value {saldoClass}">{heroValue}</div>
      <div class="hero-sub">{heroSub}</div>
      {#if saldoSeries.length > 1}
        <div class="hero-spark">
          <Sparkline values={saldoSeries} width={220} height={40} fill color="var(--accent)" />
        </div>
      {/if}
    </div>

    <!-- Breakdown: every input to the saldo, nothing hidden. -->
    <div class="grid">
      <div class="card">
        <div class="card-label">Contexto poupado</div>
        <div class="card-value pos">{fmt(data.savedTokens)}</div>
        <div class="card-note">{data.savingEvents} memórias reusadas (proxy)</div>
      </div>
      <div class="card">
        <div class="card-label">Haiku gasto</div>
        <div class="card-value neg">{fmt(data.haikuTokens)}</div>
        <div class="card-note">{fmt(data.haikuInputTokens)} in · {fmt(data.haikuOutputTokens)} out · {data.spendCalls} calls</div>
      </div>
      <div class="card">
        <div class="card-label">Contexto injetado</div>
        <div class="card-value">{fmt(data.injectedTokens)}</div>
        <div class="card-note">{data.injectionItems} itens (ocupação)</div>
      </div>
      <div class="card">
        <div class="card-label">Memória usada</div>
        <div class="card-value">{usedPct}%</div>
        <div class="card-note">injeções tocadas no turno seguinte</div>
      </div>
    </div>

    <!-- Budget ceiling: protects before spend. -->
    {#if data.budget.perDayLimit > 0 || data.budget.perSessionLimit > 0}
      <div class="budget" class:budget-hit={data.budget.dayExceeded || data.budget.pausedSessions > 0}>
        <div class="budget-head">
          <span class="budget-label">TETO DE ORÇAMENTO (HAIKU)</span>
          {#if data.budget.dayExceeded}
            <span class="budget-flag">teto diário atingido — background pausado</span>
          {:else if data.budget.pausedSessions > 0}
            <span class="budget-flag">{data.budget.pausedSessions} sessão(ões) pausada(s)</span>
          {/if}
        </div>
        {#if data.budget.perDayLimit > 0}
          <div class="budget-bar"><div class="budget-fill" style="width:{budgetPct}%"></div></div>
          <div class="budget-note">{fmt(data.budget.daySpent)} / {fmt(data.budget.perDayLimit)} tokens hoje ({budgetPct}%)
            {#if data.budget.perSessionLimit > 0} · {fmt(data.budget.perSessionLimit)}/sessão{/if}
          </div>
        {/if}
      </div>
    {/if}

    <p class="baseline">
      <strong>Como calculamos:</strong> o "contexto poupado" é uma estimativa por
      <em>proxy de substituição</em> — só conta quando uma memória injetada é tocada por um tool use
      seguinte (co-ocorrência de arquivos/conceitos), creditando um piso conservador. O Haiku gasto é
      <em>medido</em> (tokens reais do provider). Estimativa, nunca alvo: o número é o que sai da medição.
    </p>
  {/if}
</div>

<style>
  .economy { max-width: 920px; }
  .head { display:flex; align-items:center; justify-content:space-between; margin-bottom:20px; }
  .head h2 { font-size:18px; font-weight:700; color:var(--text-primary); margin:0; }
  .windows { display:flex; gap:4px; }
  .windows button {
    padding:6px 12px; background:transparent; border:1px solid var(--border); color:var(--text-muted);
    font-family:var(--font-ui); font-size:10px; font-weight:700; text-transform:uppercase;
    letter-spacing:0.06em; cursor:pointer;
  }
  .windows button.active { color:var(--accent); border-color:var(--accent); background:var(--accent-muted); }

  .msg { padding:32px; text-align:center; font-family:var(--font-mono); font-size:12px; color:var(--text-muted); }
  .msg.err { color:var(--danger, #ef4444); }

  .cold {
    padding:14px 18px; margin-bottom:18px; border:1px solid var(--border);
    background:var(--bg-secondary); color:var(--text-dim); font-size:13px; line-height:1.6;
  }

  .hero {
    border:1px solid var(--border); background:var(--bg-secondary); padding:24px;
    margin-bottom:16px; position:relative;
  }
  .hero-label {
    font-family:var(--font-mono); font-size:10px; letter-spacing:0.12em; color:var(--text-muted);
    text-transform:uppercase; display:flex; align-items:center; gap:8px;
  }
  .conf {
    font-size:9px; border:1px solid var(--border); padding:1px 6px; color:var(--text-dim);
    letter-spacing:0.06em;
  }
  .hero-value { font-family:var(--font-mono); font-size:46px; font-weight:700; line-height:1.1; margin:6px 0; }
  .hero-value.pos { color:var(--success, #10b981); }
  .hero-value.neg { color:var(--danger, #ef4444); }
  .hero-value.zero { color:var(--text-secondary); }
  .hero-sub { font-size:12px; color:var(--text-muted); }
  .hero-spark { position:absolute; right:24px; top:24px; opacity:0.9; }

  .grid { display:grid; grid-template-columns:repeat(auto-fit, minmax(180px, 1fr)); gap:12px; margin-bottom:20px; }
  .card { border:1px solid var(--border); background:var(--bg-card, var(--bg-secondary)); padding:16px; }
  .card-label {
    font-family:var(--font-ui); font-size:10px; font-weight:700; text-transform:uppercase;
    letter-spacing:0.06em; color:var(--text-muted); margin-bottom:8px;
  }
  .card-value { font-family:var(--font-mono); font-size:26px; font-weight:700; color:var(--text-primary); }
  .card-value.pos { color:var(--success, #10b981); }
  .card-value.neg { color:var(--danger, #ef4444); }
  .card-note { font-size:11px; color:var(--text-muted); margin-top:6px; }

  .budget { border:1px solid var(--border); background:var(--bg-secondary); padding:16px; margin-bottom:20px; }
  .budget-hit { border-color:var(--danger, #ef4444); }
  .budget-head { display:flex; align-items:center; justify-content:space-between; gap:12px; margin-bottom:10px; }
  .budget-label { font-family:var(--font-mono); font-size:10px; letter-spacing:0.12em; color:var(--text-muted); text-transform:uppercase; }
  .budget-flag { font-size:11px; color:var(--danger, #ef4444); font-weight:600; }
  .budget-bar { height:8px; background:var(--bg-card, var(--bg-primary)); border:1px solid var(--border); overflow:hidden; }
  .budget-fill { height:100%; background:var(--accent); transition:width 0.3s; }
  .budget-note { font-size:11px; color:var(--text-muted); margin-top:6px; }

  .baseline { font-size:12px; line-height:1.7; color:var(--text-dim); border-top:1px solid var(--border); padding-top:16px; }
  .baseline strong { color:var(--text-secondary); }
</style>
