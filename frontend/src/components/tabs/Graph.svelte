<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api } from '../../lib/api';

  let canvas: HTMLCanvasElement;
  let stats: any = null;
  let loading = true;
  let nodeCount = 0;
  let edgeCount = 0;
  let hoveredNode: any = null;
  let animFrame: number;

  // Physics simulation
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
    decision: '#eab308', pattern: '#a78bfa', library: '#f472b6', person: '#22c55e',
    project: '#c8933a', component: '#38bdf8', process: '#8b5cf6',
  };

  function getColor(type: string): string { return typeColors[type] || '#6a6a6e'; }

  onMount(async () => {
    try {
      const [s, g] = await Promise.all([api.graphStats(), api.graphAll()]) as any[];
      stats = s;
      nodeCount = s?.totalNodes || 0;
      edgeCount = s?.totalEdges || 0;

      if (g?.nodes?.length > 0) {
        buildSimulation(g.nodes, g.edges || []);
        startSimulation();
      }
    } catch (e) { console.error(e); }
    loading = false;
  });

  onDestroy(() => { if (animFrame) cancelAnimationFrame(animFrame); });

  function buildSimulation(rawNodes: any[], rawEdges: any[]) {
    const nodeMap = new Map<string, number>();
    const edgeCounts = new Map<string, number>();

    // Count edges per node
    for (const e of rawEdges) {
      const s = e.sourceNodeId || e.SourceNodeID || '';
      const t = e.targetNodeId || e.TargetNodeID || '';
      edgeCounts.set(s, (edgeCounts.get(s) || 0) + 1);
      edgeCounts.set(t, (edgeCounts.get(t) || 0) + 1);
    }

    // Build nodes with random positions
    const w = 1000, h = 800;
    nodes = rawNodes.map((n: any, i: number) => {
      const id = n.id || n.ID || '';
      nodeMap.set(id, i);
      const ec = edgeCounts.get(id) || 0;
      return {
        id, type: n.type || n.Type || 'other', name: n.name || n.Name || id,
        x: w/2 + (Math.random() - 0.5) * w * 0.8,
        y: h/2 + (Math.random() - 0.5) * h * 0.8,
        vx: 0, vy: 0, edges: ec,
        radius: Math.max(3, Math.min(16, 3 + ec * 1.5)),
      };
    });

    // Build edges
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
    const maxIterations = 500;

    function tick() {
      if (iteration > maxIterations) { draw(); return; }
      iteration++;

      const repulsion = 3000;
      const attraction = 0.003;
      const damping = 0.9;
      const centerGravity = 0.003;
      const cx = 500, cy = 400;

      // Repulsion (node-node)
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

      // Attraction (edges)
      for (const e of edges) {
        const s = nodes[e.source], t = nodes[e.target];
        const dx = t.x - s.x, dy = t.y - s.y;
        const dist = Math.sqrt(dx*dx + dy*dy);
        const force = dist * attraction;
        const fx = (dx / Math.max(1, dist)) * force;
        const fy = (dy / Math.max(1, dist)) * force;
        s.vx += fx; s.vy += fy;
        t.vx -= fx; t.vy -= fy;
      }

      // Center gravity + apply velocity
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
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    const w = canvas.width, h = canvas.height;

    ctx.clearRect(0, 0, w, h);
    ctx.save();
    ctx.translate(panX + w/2, panY + h/2);
    ctx.scale(zoom, zoom);
    ctx.translate(-500, -400);

    // Draw edges
    ctx.lineWidth = 0.5;
    ctx.strokeStyle = 'rgba(100,100,100,0.25)';
    ctx.beginPath();
    for (const e of edges) {
      const s = nodes[e.source], t = nodes[e.target];
      ctx.moveTo(s.x, s.y);
      ctx.lineTo(t.x, t.y);
    }
    ctx.stroke();

    // Draw nodes
    for (const n of nodes) {
      ctx.beginPath();
      ctx.arc(n.x, n.y, n.radius, 0, Math.PI * 2);
      ctx.fillStyle = getColor(n.type);
      ctx.globalAlpha = n === hoveredNode ? 1 : 0.85;
      ctx.fill();
      ctx.globalAlpha = 1;

      // Label for large nodes or hovered
      if (n.radius > 6 || n === hoveredNode) {
        ctx.fillStyle = '#f4f4f5';
        ctx.font = `${Math.max(8, n.radius * 0.9)}px Manrope, sans-serif`;
        ctx.textAlign = 'center';
        ctx.fillText(n.name.length > 20 ? n.name.slice(0, 18) + '..' : n.name, n.x, n.y - n.radius - 4);
      }
    }

    ctx.restore();
  }

  function onMouseMove(e: MouseEvent) {
    if (dragging) {
      panX = panStartX + (e.clientX - dragStartX);
      panY = panStartY + (e.clientY - dragStartY);
      draw();
      return;
    }
    // Hit test for hover
    const rect = canvas.getBoundingClientRect();
    const mx = (e.clientX - rect.left - panX - canvas.width/2) / zoom + 500;
    const my = (e.clientY - rect.top - panY - canvas.height/2) / zoom + 400;
    hoveredNode = null;
    for (const n of nodes) {
      const dx = n.x - mx, dy = n.y - my;
      if (dx*dx + dy*dy < (n.radius + 4) * (n.radius + 4)) {
        hoveredNode = n;
        break;
      }
    }
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
    zoom = Math.max(0.2, Math.min(5, zoom));
    draw();
  }

  function resetView() { zoom = 1; panX = 0; panY = 0; draw(); }
</script>

<div class="graph-container">
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
    <!-- Canvas -->
    <div class="canvas-wrapper">
      <canvas
        bind:this={canvas}
        width={1000} height={800}
        on:mousemove={onMouseMove}
        on:mousedown={onMouseDown}
        on:mouseup={onMouseUp}
        on:mouseleave={onMouseUp}
        on:wheel={onWheel}
      ></canvas>

      {#if hoveredNode}
        <div class="tooltip">
          <span class="tooltip-type" style="color:{getColor(hoveredNode.type)}">{hoveredNode.type}</span>
          <span class="tooltip-name">{hoveredNode.name}</span>
          <span class="tooltip-edges mono">{hoveredNode.edges} connections</span>
        </div>
      {/if}
    </div>

    <!-- Legend -->
    <div class="legend">
      {#each Object.entries(typeColors) as [type, color]}
        <div class="legend-item">
          <div class="legend-dot" style="background:{color}"></div>
          <span>{type}</span>
        </div>
      {/each}
    </div>

    <!-- Breakdown Tables -->
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
  .graph-container { display: flex; flex-direction: column; }
  .graph-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 20px; }
  .graph-header h3 { font-family: var(--font-display); font-size: 16px; font-weight: 600; letter-spacing: -0.02em; }
  .graph-controls { display: flex; align-items: center; gap: 12px; }
  .stat-mini { font-size: 11px; color: var(--text-muted); }

  .canvas-wrapper {
    position: relative;
    background: var(--bg-card);
    border: 1px solid var(--border);
    margin-bottom: 20px;
    overflow: hidden;
  }
  .canvas-wrapper:hover { border-color: var(--accent); }
  canvas { display: block; width: 100%; height: 700px; cursor: grab; background: #050505; }
  canvas:active { cursor: grabbing; }

  .canvas-shell { width: 100%; height: 700px; background: var(--bg-card); border: 1px solid var(--border); }
  .skeleton { animation: pulse 1.5s ease-in-out infinite; }
  @keyframes pulse { 0%, 100% { opacity: 0.3; } 50% { opacity: 0.6; } }

  .tooltip {
    position: absolute;
    top: 12px; left: 12px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    padding: 8px 14px;
    display: flex; flex-direction: column; gap: 2px;
    pointer-events: none;
  }
  .tooltip-type { font-size: 10px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.08em; font-family: var(--font-ui); }
  .tooltip-name { font-size: 14px; font-weight: 600; color: var(--text-primary); font-family: var(--font-body); }
  .tooltip-edges { font-size: 11px; color: var(--text-muted); }

  .legend {
    display: flex; flex-wrap: wrap; gap: 12px; margin-bottom: 24px;
    padding: 12px 16px;
    background: var(--bg-card); border: 1px solid var(--border);
  }
  .legend-item { display: flex; align-items: center; gap: 6px; font-size: 11px; color: var(--text-muted); font-family: var(--font-ui); text-transform: uppercase; letter-spacing: 0.06em; }
  .legend-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
  .legend-dot-inline { display: inline-block; width: 6px; height: 6px; border-radius: 50%; margin-right: 6px; }

  .breakdown-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
  @media (max-width: 700px) { .breakdown-grid { grid-template-columns: 1fr; } }
  .breakdown-card { background: var(--bg-card); border: 1px solid var(--border); padding: 20px; }
  .breakdown-card:hover { border-color: var(--accent); box-shadow: var(--shadow-hover); }
  .breakdown-header { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; }
  .breakdown-title { font-family: var(--font-ui); font-size: 10px; font-weight: 700; color: var(--accent); text-transform: uppercase; letter-spacing: 0.12em; }
</style>
