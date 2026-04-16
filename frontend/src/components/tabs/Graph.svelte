<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api } from '../../lib/api';

  let canvasEl: HTMLCanvasElement;
  let wrapper: HTMLDivElement;
  let stats: any = null;
  let loading = true;
  let nodeCount = 0;
  let edgeCount = 0;
  let hoveredNode: any = null;
  let animFrame: number;
  let dpr = 1;
  let W = 0, H = 0;

  interface SimNode {
    id: string; type: string; name: string;
    x: number; y: number; vx: number; vy: number;
    edges: number; radius: number;
  }
  interface SimEdge { source: number; target: number; type: string; weight: number; }

  let nodes: SimNode[] = [];
  let edges: SimEdge[] = [];
  let zoom = 1;
  let panX = 0, panY = 0;
  let dragging = false;
  let dragStartX = 0, dragStartY = 0;
  let panStartX = 0, panStartY = 0;

  const typeColors: Record<string, string> = {
    concept: '#e8a065', file: '#5ba3d9', function: '#4ecdc4', error: '#ef4444',
    decision: '#eab308', pattern: '#a78bfa', library: '#f472b6', person: '#34d399',
    project: '#c8933a', component: '#38bdf8', process: '#8b5cf6',
  };

  function getColor(type: string): string { return typeColors[type] || '#555'; }

  onMount(async () => {
    dpr = window.devicePixelRatio || 1;
    try {
      const [s, g] = await Promise.all([api.graphStats(), api.graphAll()]) as any[];
      stats = s;
      nodeCount = s?.totalNodes || 0;
      edgeCount = s?.totalEdges || 0;
      if (g?.nodes?.length > 0) {
        buildSimulation(g.nodes, g.edges || []);
        resizeCanvas();
        startSimulation();
      }
    } catch (e) { console.error(e); }
    loading = false;
  });

  onDestroy(() => { if (animFrame) cancelAnimationFrame(animFrame); });

  function resizeCanvas() {
    if (!canvasEl || !wrapper) return;
    const rect = wrapper.getBoundingClientRect();
    W = rect.width || 800;
    H = 700;
    // Set backing store size for HiDPI — do NOT pre-scale the context here
    canvasEl.width = W * dpr;
    canvasEl.height = H * dpr;
    canvasEl.style.width = W + 'px';
    canvasEl.style.height = H + 'px';
  }

  function buildSimulation(rawNodes: any[], rawEdges: any[]) {
    const nodeMap = new Map<string, number>();
    const edgeCounts = new Map<string, number>();

    for (const e of rawEdges) {
      const s = e.sourceNodeId || e.SourceNodeID || '';
      const t = e.targetNodeId || e.TargetNodeID || '';
      edgeCounts.set(s, (edgeCounts.get(s) || 0) + 1);
      edgeCounts.set(t, (edgeCounts.get(t) || 0) + 1);
    }

    nodes = rawNodes.map((n: any, i: number) => {
      const id = n.id || n.ID || '';
      nodeMap.set(id, i);
      const ec = edgeCounts.get(id) || 0;
      return {
        id, type: n.type || n.Type || 'other', name: n.name || n.Name || id,
        x: 500 + (Math.random() - 0.5) * 800,
        y: 400 + (Math.random() - 0.5) * 600,
        vx: 0, vy: 0, edges: ec,
        radius: Math.max(3, Math.min(20, 3 + ec * 2)),
      };
    });

    edges = [];
    for (const e of rawEdges) {
      const si = nodeMap.get(e.sourceNodeId || e.SourceNodeID || '');
      const ti = nodeMap.get(e.targetNodeId || e.TargetNodeID || '');
      if (si !== undefined && ti !== undefined) {
        edges.push({ source: si, target: ti, type: e.type || e.Type || '', weight: e.weight || e.Weight || 0.5 });
      }
    }
  }

  function startSimulation() {
    let iteration = 0;
    const maxIterations = 600;

    function tick() {
      if (iteration > maxIterations) { draw(); return; }
      iteration++;

      const repulsion = 5000;
      const attraction = 0.002;
      const damping = 0.88;
      const centerGravity = 0.002;
      const cx = 500, cy = 350;

      // Repulsion
      for (let i = 0; i < nodes.length; i++) {
        for (let j = i + 1; j < nodes.length; j++) {
          const dx = nodes[j].x - nodes[i].x;
          const dy = nodes[j].y - nodes[i].y;
          const dist = Math.max(1, Math.sqrt(dx*dx + dy*dy));
          const force = repulsion / (dist * dist);
          const fx = (dx / dist) * force;
          const fy = (dy / dist) * force;
          nodes[i].vx -= fx; nodes[i].vy -= fy;
          nodes[j].vx += fx; nodes[j].vy += fy;
        }
      }

      // Attraction
      for (const e of edges) {
        const s = nodes[e.source], t = nodes[e.target];
        const dx = t.x - s.x, dy = t.y - s.y;
        const dist = Math.max(1, Math.sqrt(dx*dx + dy*dy));
        const force = dist * attraction;
        const fx = (dx / dist) * force;
        const fy = (dy / dist) * force;
        s.vx += fx; s.vy += fy;
        t.vx -= fx; t.vy -= fy;
      }

      // Apply
      for (const n of nodes) {
        n.vx += (cx - n.x) * centerGravity;
        n.vy += (cy - n.y) * centerGravity;
        n.vx *= damping; n.vy *= damping;
        n.x += n.vx; n.y += n.vy;
      }

      draw();
      animFrame = requestAnimationFrame(tick);
    }

    animFrame = requestAnimationFrame(tick);
  }

  function draw() {
    if (!canvasEl) return;
    const ctx = canvasEl.getContext('2d');
    if (!ctx) return;

    // Clear with DPR-scaled dimensions
    ctx.setTransform(1, 0, 0, 1, 0, 0);
    ctx.clearRect(0, 0, canvasEl.width, canvasEl.height);

    // Apply DPR scaling, then pan + zoom
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.save();
    ctx.translate(panX + W/2, panY + H/2);
    ctx.scale(zoom, zoom);
    ctx.translate(-500, -350);

    // Edges — thin glowing lines
    for (const e of edges) {
      const s = nodes[e.source], t = nodes[e.target];
      ctx.beginPath();
      ctx.moveTo(s.x, s.y);
      ctx.lineTo(t.x, t.y);
      ctx.strokeStyle = 'rgba(200, 147, 58, 0.08)';
      ctx.lineWidth = 0.8;
      ctx.stroke();
    }

    // Highlight edges for hovered node
    if (hoveredNode) {
      const hi = nodes.indexOf(hoveredNode);
      for (const e of edges) {
        if (e.source === hi || e.target === hi) {
          const s = nodes[e.source], t = nodes[e.target];
          ctx.beginPath();
          ctx.moveTo(s.x, s.y);
          ctx.lineTo(t.x, t.y);
          ctx.strokeStyle = 'rgba(200, 147, 58, 0.5)';
          ctx.lineWidth = 1.5;
          ctx.stroke();
        }
      }
    }

    // Nodes — glowing circles
    for (const n of nodes) {
      const isHovered = n === hoveredNode;
      const color = getColor(n.type);

      // Glow
      if (n.radius > 5 || isHovered) {
        ctx.beginPath();
        ctx.arc(n.x, n.y, n.radius * (isHovered ? 3 : 2), 0, Math.PI * 2);
        const grad = ctx.createRadialGradient(n.x, n.y, 0, n.x, n.y, n.radius * (isHovered ? 3 : 2));
        grad.addColorStop(0, color + (isHovered ? '40' : '15'));
        grad.addColorStop(1, color + '00');
        ctx.fillStyle = grad;
        ctx.fill();
      }

      // Core
      ctx.beginPath();
      ctx.arc(n.x, n.y, n.radius, 0, Math.PI * 2);
      ctx.fillStyle = color;
      ctx.globalAlpha = isHovered ? 1 : 0.9;
      ctx.fill();
      ctx.globalAlpha = 1;

      // Label
      if (n.radius > 7 || isHovered) {
        ctx.fillStyle = '#f4f4f5';
        ctx.font = `600 ${Math.max(9, n.radius)}px Manrope, sans-serif`;
        ctx.textAlign = 'center';
        ctx.textBaseline = 'bottom';
        const label = n.name.length > 24 ? n.name.slice(0, 22) + '..' : n.name;
        ctx.fillText(label, n.x, n.y - n.radius - 5);
      }
    }

    ctx.restore(); // undo pan+zoom save
  }

  function onMouseMove(e: MouseEvent) {
    if (!canvasEl) return;
    if (dragging) {
      panX = panStartX + (e.clientX - dragStartX);
      panY = panStartY + (e.clientY - dragStartY);
      draw();
      return;
    }
    const rect = canvasEl.getBoundingClientRect();
    const mx = (e.clientX - rect.left - panX - W/2) / zoom + 500;
    const my = (e.clientY - rect.top - panY - H/2) / zoom + 350;
    hoveredNode = null;
    for (const n of nodes) {
      const dx = n.x - mx, dy = n.y - my;
      if (dx*dx + dy*dy < (n.radius + 5) * (n.radius + 5)) {
        hoveredNode = n;
        break;
      }
    }
    if (canvasEl) canvasEl.style.cursor = hoveredNode ? 'pointer' : 'grab';
    draw();
  }

  function onMouseDown(e: MouseEvent) {
    dragging = true;
    dragStartX = e.clientX; dragStartY = e.clientY;
    panStartX = panX; panStartY = panY;
  }
  function onMouseUp() { dragging = false; }

  function onWheel(e: WheelEvent) {
    e.preventDefault();
    zoom *= e.deltaY > 0 ? 0.92 : 1.08;
    zoom = Math.max(0.1, Math.min(8, zoom));
    draw();
  }

  function resetView() { zoom = 1; panX = 0; panY = 0; draw(); }
</script>

<div class="graph-page">
  <div class="graph-header">
    <div>
      <div class="gold-line"></div>
      <h3>Knowledge Graph</h3>
    </div>
    <div class="graph-controls">
      <span class="stat-mini mono">{nodeCount} nodes</span>
      <span class="stat-mini mono">{edgeCount} edges</span>
      <button class="btn" on:click={resetView}>Reset</button>
    </div>
  </div>

  {#if loading}
    <div class="canvas-shell skeleton"></div>
  {:else if nodes.length === 0}
    <div class="empty-state">
      <div class="icon" style="font-size:32px; opacity:0.15">&#9670;</div>
      <p>No graph data yet</p>
    </div>
  {:else}
    <div class="canvas-wrapper" bind:this={wrapper}>
      <canvas
        bind:this={canvasEl}
        on:mousemove={onMouseMove}
        on:mousedown={onMouseDown}
        on:mouseup={onMouseUp}
        on:mouseleave={onMouseUp}
        on:wheel|preventDefault={onWheel}
      ></canvas>

      {#if hoveredNode}
        <div class="tooltip">
          <span class="tooltip-type" style="color:{getColor(hoveredNode.type)}">{hoveredNode.type}</span>
          <span class="tooltip-name">{hoveredNode.name}</span>
          <span class="tooltip-edges mono">{hoveredNode.edges} connections</span>
        </div>
      {/if}
    </div>

    <div class="legend">
      {#each Object.entries(typeColors) as [type, color]}
        <div class="legend-item">
          <div class="legend-dot" style="background:{color}"></div>
          <span>{type}</span>
        </div>
      {/each}
    </div>

    <div class="breakdown-grid">
      {#if stats?.nodesByType && Object.keys(stats.nodesByType).length > 0}
        <div class="breakdown-card">
          <div class="breakdown-header">
            <div class="gold-line" style="margin-bottom:0"></div>
            <span class="breakdown-title">NODES BY TYPE</span>
          </div>
          <table>
            <thead><tr><th>TYPE</th><th style="text-align:right">COUNT</th></tr></thead>
            <tbody>
              {#each Object.entries(stats.nodesByType).sort((a,b) => Number(b[1]) - Number(a[1])) as [type, count]}
                <tr>
                  <td><span class="legend-dot-inline" style="background:{getColor(type)}"></span> {type}</td>
                  <td class="mono" style="text-align:right">{count}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
      {#if stats?.edgesByType && Object.keys(stats.edgesByType).length > 0}
        <div class="breakdown-card">
          <div class="breakdown-header">
            <div class="gold-line" style="margin-bottom:0"></div>
            <span class="breakdown-title">EDGES BY TYPE</span>
          </div>
          <table>
            <thead><tr><th>TYPE</th><th style="text-align:right">COUNT</th></tr></thead>
            <tbody>
              {#each Object.entries(stats.edgesByType).sort((a,b) => Number(b[1]) - Number(a[1])) as [type, count]}
                <tr>
                  <td>{type}</td>
                  <td class="mono" style="text-align:right">{count}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .graph-page { display: flex; flex-direction: column; }
  .graph-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 16px; }
  .graph-header h3 { font-family: var(--font-display); font-size: 16px; font-weight: 600; }
  .graph-controls { display: flex; align-items: center; gap: 12px; }
  .stat-mini { font-size: 11px; color: var(--text-muted); }

  .canvas-wrapper {
    position: relative;
    background: #030303;
    border: 1px solid var(--border);
    margin-bottom: 16px;
    overflow: hidden;
  }
  .canvas-wrapper:hover { border-color: var(--accent); }
  canvas { display: block; cursor: grab; }
  canvas:active { cursor: grabbing; }

  .canvas-shell { width: 100%; height: 700px; background: var(--bg-card); border: 1px solid var(--border); }
  .skeleton { animation: pulse 1.5s ease-in-out infinite; }
  @keyframes pulse { 0%, 100% { opacity: 0.3; } 50% { opacity: 0.6; } }

  .tooltip {
    position: absolute;
    top: 12px; left: 12px;
    background: rgba(3,3,3,0.9);
    border: 1px solid var(--accent);
    padding: 10px 16px;
    display: flex; flex-direction: column; gap: 2px;
    pointer-events: none;
    backdrop-filter: blur(4px);
  }
  .tooltip-type { font-size: 10px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.1em; font-family: var(--font-ui); }
  .tooltip-name { font-size: 15px; font-weight: 600; color: var(--text-primary); font-family: var(--font-display); }
  .tooltip-edges { font-size: 11px; color: var(--text-muted); }

  .legend {
    display: flex; flex-wrap: wrap; gap: 14px; margin-bottom: 20px;
    padding: 10px 16px;
    background: var(--bg-card); border: 1px solid var(--border);
  }
  .legend-item { display: flex; align-items: center; gap: 6px; font-size: 10px; color: var(--text-muted); font-family: var(--font-ui); text-transform: uppercase; letter-spacing: 0.06em; }
  .legend-dot { width: 8px; height: 8px; border-radius: 50%; }
  .legend-dot-inline { display: inline-block; width: 6px; height: 6px; border-radius: 50%; margin-right: 6px; }

  .breakdown-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
  @media (max-width: 700px) { .breakdown-grid { grid-template-columns: 1fr; } }
  .breakdown-card { background: var(--bg-card); border: 1px solid var(--border); padding: 20px; }
  .breakdown-card:hover { border-color: var(--accent); box-shadow: var(--shadow-hover); }
  .breakdown-header { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; }
  .breakdown-title { font-family: var(--font-ui); font-size: 10px; font-weight: 700; color: var(--accent); text-transform: uppercase; letter-spacing: 0.12em; }
</style>
