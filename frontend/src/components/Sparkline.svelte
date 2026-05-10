<script lang="ts">
  // Sparkline minimalista — 1 polyline + opcional fill embaixo. Sem
  // eixos, sem labels. Usada pra mostrar tendência de métricas no
  // header de cards (ex: calls/min do pipeline).
  let {
    values,
    width = 80,
    height = 20,
    color = 'currentColor',
    fill = false,
  }: {
    values: number[];
    width?: number;
    height?: number;
    color?: string;
    fill?: boolean;
  } = $props();

  // path SVG e ponto final (pra desenhar o dot do "agora")
  let viewBox = $derived(`0 0 ${width} ${height}`);
  let computed = $derived.by(() => {
    if (!values || values.length === 0) {
      return { line: '', area: '', last: null as { x: number; y: number } | null };
    }
    const max = Math.max(1, ...values);
    const min = Math.min(0, ...values);
    const range = Math.max(1, max - min);
    const stepX = values.length > 1 ? width / (values.length - 1) : 0;
    const pad = 2; // margem vertical pra dot não cortar
    const usable = height - pad * 2;
    const points = values.map((v, i) => {
      const x = i * stepX;
      const y = height - pad - ((v - min) / range) * usable;
      return { x, y };
    });
    const line = points.map((p, i) => `${i === 0 ? 'M' : 'L'}${p.x.toFixed(1)},${p.y.toFixed(1)}`).join(' ');
    const area = fill && points.length > 1
      ? `${line} L${points[points.length - 1].x.toFixed(1)},${height} L0,${height} Z`
      : '';
    return { line, area, last: points[points.length - 1] };
  });
</script>

<svg width={width} height={height} viewBox={viewBox} aria-hidden="true">
  {#if computed.area}
    <path d={computed.area} fill={color} fill-opacity="0.10" />
  {/if}
  {#if computed.line}
    <path d={computed.line} fill="none" stroke={color} stroke-width="1.2" stroke-linejoin="round" stroke-linecap="round" />
  {/if}
  {#if computed.last}
    <circle cx={computed.last.x} cy={computed.last.y} r="1.6" fill={color} />
  {/if}
</svg>
