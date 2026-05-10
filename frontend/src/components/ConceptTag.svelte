<script lang="ts">
  // Tag colorida reutilizável. A cor vem de um hash determinístico do
  // texto, então o mesmo concept ("auth", "decay") pinta sempre da
  // mesma cor em todas as views — ajuda o olho a reconhecer padrões.
  // Quando `onClick` está presente, vira interativo (filtro toggle).
  let { label, active = false, onClick }: { label: string; active?: boolean; onClick?: (l: string) => void } = $props();

  // Hash FNV-1a curto. Usado pra index num conjunto fixo de hues
  // de modo que vizinhos no alfabeto não saiam parecidos.
  function hashCode(s: string): number {
    let h = 2166136261 >>> 0;
    for (let i = 0; i < s.length; i++) {
      h ^= s.charCodeAt(i);
      h = Math.imul(h, 16777619) >>> 0;
    }
    return h;
  }

  // Paleta calibrada pra contrastar tanto no light quanto no dark theme.
  // Todas com saturation média e lightness intermediária — visível em
  // ambos os fundos sem precisar trocar dinamicamente.
  const palette = [
    { fg: '#e8a065', bg: 'rgba(232,160,101,0.10)', border: 'rgba(232,160,101,0.35)' }, // amber
    { fg: '#5ba3d9', bg: 'rgba(91,163,217,0.10)', border: 'rgba(91,163,217,0.35)' },   // blue
    { fg: '#4ecdc4', bg: 'rgba(78,205,196,0.10)', border: 'rgba(78,205,196,0.35)' },   // teal
    { fg: '#a78bfa', bg: 'rgba(167,139,250,0.10)', border: 'rgba(167,139,250,0.35)' }, // violet
    { fg: '#f472b6', bg: 'rgba(244,114,182,0.10)', border: 'rgba(244,114,182,0.35)' }, // pink
    { fg: '#34d399', bg: 'rgba(52,211,153,0.10)', border: 'rgba(52,211,153,0.35)' },   // green
    { fg: '#eab308', bg: 'rgba(234,179,8,0.10)', border: 'rgba(234,179,8,0.35)' },     // yellow
    { fg: '#38bdf8', bg: 'rgba(56,189,248,0.10)', border: 'rgba(56,189,248,0.35)' },   // sky
    { fg: '#8b5cf6', bg: 'rgba(139,92,246,0.10)', border: 'rgba(139,92,246,0.35)' },   // purple
    { fg: '#fb923c', bg: 'rgba(251,146,60,0.10)', border: 'rgba(251,146,60,0.35)' },   // orange
  ];

  let color = $derived(palette[hashCode(label || '') % palette.length]);
</script>

{#if onClick}
  <button
    type="button"
    class="ct"
    class:ct-active={active}
    style="--ct-fg:{color.fg};--ct-bg:{color.bg};--ct-border:{color.border}"
    onclick={() => onClick?.(label)}
  >{label}</button>
{:else}
  <span
    class="ct"
    style="--ct-fg:{color.fg};--ct-bg:{color.bg};--ct-border:{color.border}"
  >{label}</span>
{/if}

<style>
  .ct {
    display: inline-flex;
    align-items: center;
    padding: 2px 10px;
    border: 1px solid var(--ct-border);
    background: var(--ct-bg);
    color: var(--ct-fg);
    font-family: var(--font-ui);
    font-size: 10px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    border-radius: 0;
    line-height: 1.6;
  }
  button.ct {
    cursor: pointer;
    transition: all 0.15s var(--ease);
  }
  button.ct:hover {
    background: var(--ct-fg);
    color: #ffffff;
    border-color: var(--ct-fg);
  }
  button.ct.ct-active {
    background: var(--ct-fg);
    color: #ffffff;
    border-color: var(--ct-fg);
    box-shadow: 0 0 0 1px var(--ct-fg);
  }
</style>
