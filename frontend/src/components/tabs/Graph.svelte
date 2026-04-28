<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api } from '../../lib/api';
  import { createPoller } from '../../lib/poller';

  let canvasEl: HTMLCanvasElement;
  let stats: any = null;
  let loading = true;
  let nodeCount = 0;
  let edgeCount = 0;
  let hoveredNode: any = null;
  let animFrame: number;

  // Simulation space: fixed coords, camera maps to canvas
  const SIM_W = 1200, SIM_H = 900;

  interface SimNode {
    id: string; type: string; name: string;
    x: number; y: number; vx: number; vy: number;
    edges: number; radius: number;
  }
  interface SimEdge { source: number; target: number; type: string; }

  let nodes: SimNode[] = [];
  let edges: SimEdge[] = [];
  let zoom = 1;
  let panX = 0, panY = 0;
  let dragging = false;
  let dragStartX = 0, dragStartY = 0;
  let panStartX = 0, panStartY = 0;
  let stopPoll: (() => void) | undefined;

  const typeColors: Record<string, string> = {
    concept: '#e8a065', file: '#5ba3d9', function: '#4ecdc4', error: '#ef4444',
    decision: '#eab308', pattern: '#a78bfa', library: '#f472b6', person: '#34d399',
    project: '#c8933a', component: '#38bdf8', process: '#8b5cf6',
  };
  function getColor(type: string): string { return typeColors[type] || '#555'; }

  onMount(async () => {
    await refresh(true);
    // Poll every 10s; only rebuild the simulation when node/edge counts actually change.
    stopPoll = createPoller(() => refresh(false), 10000);
  });

  onDestroy(() => {
    if (animFrame) cancelAnimationFrame(animFrame);
    stopPoll?.();
  });

  async function refresh(initial: boolean) {
    try {
      const [s, g] = await Promise.all([api.graphStats(), api.graphAll()]) as any[];
      const newNodeCount = s?.totalNodes || 0;
      const newEdgeCount = s?.totalEdges || 0;
      const structureChanged = newNodeCount !== nodeCount || newEdgeCount !== edgeCount;
      stats = s;
      nodeCount = newNodeCount;
      edgeCount = newEdgeCount;
      if ((initial || structureChanged) && g?.nodes?.length > 0) {
        buildSimulation(g.nodes, g.edges || []);
        startSimulation();
      }
    } catch (e) { console.error(e); }
    if (initial) loading = false;
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
    // Start nodes in a circle in the middle of sim space
    const cx = SIM_W / 2, cy = SIM_H / 2;
    nodes = rawNodes.map((n: any, i: number) => {
      const id = n.id || n.ID || '';
      nodeMap.set(id, i);
      const ec = edgeCounts.get(id) || 0;
      const angle = (i / rawNodes.length) * Math.PI * 2;
      const r = 80 + Math.random() * 120;
      return {
        id, type: n.type || n.Type || 'other', name: n.name || n.Name || id,
        x: cx + Math.cos(angle) * r,
        y: cy + Math.sin(angle) * r,
        vx: (Math.random() - 0.5) * 0.5,
        vy: (Math.random() - 0.5) * 0.5,
        edges: ec,
        radius: Math.max(4, Math.min(18, 4 + ec * 1.8)),
      };
    });
    edges = [];
    for (const e of rawEdges) {
      const si = nodeMap.get(e.sourceNodeId || e.SourceNodeID || '');
      const ti = nodeMap.get(e.targetNodeId || e.TargetNodeID || '');
      if (si !== undefined && ti !== undefined)
        edges.push({ source: si, target: ti, type: e.type || e.Type || '' });
    }
  }

  function startSimulation() {
    let iter = 0;
    function tick() {
      iter++;
      const rep = 2000, att = 0.003, damp = 0.87, grav = 0.003;
      const cx = SIM_W / 2, cy = SIM_H / 2;
      for (let i = 0; i < nodes.length; i++) {
        for (let j = i + 1; j < nodes.length; j++) {
          const dx = nodes[j].x - nodes[i].x, dy = nodes[j].y - nodes[i].y;
          const d = Math.max(1, Math.sqrt(dx*dx + dy*dy));
          const f = rep / (d * d);
          const fx = (dx/d)*f, fy = (dy/d)*f;
          nodes[i].vx -= fx; nodes[i].vy -= fy;
          nodes[j].vx += fx; nodes[j].vy += fy;
        }
      }
      for (const e of edges) {
        const s = nodes[e.source], t = nodes[e.target];
        const dx = t.x - s.x, dy = t.y - s.y;
        const d = Math.max(1, Math.sqrt(dx*dx + dy*dy));
        const f = d * att;
        const fx = (dx/d)*f, fy = (dy/d)*f;
        s.vx += fx; s.vy += fy; t.vx -= fx; t.vy -= fy;
      }
      for (const n of nodes) {
        n.vx += (cx - n.x) * grav; n.vy += (cy - n.y) * grav;
        n.vx *= damp; n.vy *= damp;
        n.x += n.vx; n.y += n.vy;
        // Clamp to sim bounds
        n.x = Math.max(n.radius, Math.min(SIM_W - n.radius, n.x));
        n.y = Math.max(n.radius, Math.min(SIM_H - n.radius, n.y));
      }
      draw();
      if (iter < 500) animFrame = requestAnimationFrame(tick);
      else draw();
    }
    animFrame = requestAnimationFrame(tick);
  }

  function draw() {
    if (!canvasEl) return;
    const ctx = canvasEl.getContext('2d');
    if (!ctx) return;
    const dpr = window.devicePixelRatio || 1;
    const CW = canvasEl.clientWidth;
    const CH = canvasEl.clientHeight;

    // Ensure canvas backing store matches display size
    if (canvasEl.width !== CW * dpr || canvasEl.height !== CH * dpr) {
      canvasEl.width = CW * dpr;
      canvasEl.height = CH * dpr;
    }

    ctx.save();
    ctx.scale(dpr, dpr);
    ctx.clearRect(0, 0, CW, CH);

    // Camera: map sim space to canvas
    ctx.save();
    ctx.translate(panX + CW / 2 - (SIM_W / 2) * zoom, panY + CH / 2 - (SIM_H / 2) * zoom);
    ctx.scale(zoom, zoom);

    // Edges — always visible
    ctx.lineWidth = 0.9;
    ctx.strokeStyle = 'rgba(200,147,58,0.35)';
    ctx.beginPath();
    for (const e of edges) {
      const s = nodes[e.source], t = nodes[e.target];
      ctx.moveTo(s.x, s.y); ctx.lineTo(t.x, t.y);
    }
    ctx.stroke();

    // Hovered edges
    if (hoveredNode) {
      const hi = nodes.indexOf(hoveredNode);
      ctx.lineWidth = 1.5;
      ctx.strokeStyle = 'rgba(200,147,58,0.55)';
      ctx.beginPath();
      for (const e of edges) {
        if (e.source === hi || e.target === hi) {
          ctx.moveTo(nodes[e.source].x, nodes[e.source].y);
          ctx.lineTo(nodes[e.target].x, nodes[e.target].y);
        }
      }
      ctx.stroke();
    }

    // Nodes
    for (const n of nodes) {
      const color = getColor(n.type);
      const hov = n === hoveredNode;
      // Glow
      if (n.radius > 5 || hov) {
        const g = ctx.createRadialGradient(n.x, n.y, 0, n.x, n.y, n.radius * (hov ? 4 : 2.5));
        g.addColorStop(0, color + (hov ? '50' : '20'));
        g.addColorStop(1, color + '00');
        ctx.fillStyle = g;
        ctx.beginPath(); ctx.arc(n.x, n.y, n.radius * (hov ? 4 : 2.5), 0, Math.PI*2); ctx.fill();
      }
      // Core
      ctx.fillStyle = color;
      ctx.globalAlpha = hov ? 1 : 0.88;
      ctx.beginPath(); ctx.arc(n.x, n.y, n.radius, 0, Math.PI*2); ctx.fill();
      ctx.globalAlpha = 1;
      // Label
      if (n.radius > 7 || hov) {
        ctx.fillStyle = '#f4f4f5';
        ctx.font = `600 ${Math.max(9, n.radius * 0.85)}px Manrope, sans-serif`;
        ctx.textAlign = 'center'; ctx.textBaseline = 'bottom';
        ctx.fillText(n.name.slice(0, 22), n.x, n.y - n.radius - 4);
      }
    }

    ctx.restore();
    ctx.restore();
  }

  function getSimCoords(clientX: number, clientY: number): [number, number] {
    if (!canvasEl) return [0, 0];
    const rect = canvasEl.getBoundingClientRect();
    const CW = canvasEl.clientWidth, CH = canvasEl.clientHeight;
    const sx = (clientX - rect.left - panX - CW/2 + (SIM_W/2)*zoom) / zoom;
    const sy = (clientY - rect.top - panY - CH/2 + (SIM_H/2)*zoom) / zoom;
    return [sx, sy];
  }

  function onMouseMove(e: MouseEvent) {
    if (dragging) {
      panX = panStartX + (e.clientX - dragStartX);
      panY = panStartY + (e.clientY - dragStartY);
      draw(); return;
    }
    const [mx, my] = getSimCoords(e.clientX, e.clientY);
    hoveredNode = null;
    for (const n of nodes) {
      const dx = n.x - mx, dy = n.y - my;
      if (dx*dx + dy*dy < (n.radius+6)*(n.radius+6)) { hoveredNode = n; break; }
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
    zoom = Math.max(0.15, Math.min(8, zoom));
    draw();
  }

  function resetView() { zoom = 1; panX = 0; panY = 0; draw(); }

  let fullscreen = false;

  function toggleFullscreen() {
    fullscreen = !fullscreen;
    // Redraw after layout settles
    setTimeout(() => draw(), 50);
  }

  // ESC closes fullscreen
  function onKeyDown(e: KeyboardEvent) {
    if (e.key === 'Escape' && fullscreen) { fullscreen = false; setTimeout(() => draw(), 50); }
  }
</script>

<svelte:window on:keydown={onKeyDown} />

<div class="graph-page" class:is-fullscreen={fullscreen}>
  <div class="graph-header">
    <div>
      <div class="gold-line"></div>
      <h3>Knowledge Graph</h3>
    </div>
    <div class="graph-controls">
      <span class="stat-mini mono">{nodeCount} nodes · {edgeCount} edges</span>
      <button class="btn" on:click={resetView}>Reset</button>
      <button class="btn btn-icon" on:click={toggleFullscreen} title={fullscreen ? 'Exit fullscreen (Esc)' : 'Fullscreen'}>
        {fullscreen ? '⊡' : '⊞'}
      </button>
    </div>
  </div>

  {#if loading}
    <div class="canvas-shell skeleton"></div>
  {:else if nodes.length === 0}
    <div class="empty-state">
      <div class="icon" style="font-size:32px;opacity:0.15">&#9670;</div>
      <p>No graph data yet. Run a session and close it to populate the graph.</p>
    </div>
  {:else}
    <div class="canvas-wrapper">
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
          <span style="color:{getColor(hoveredNode.type)};font-size:10px;font-weight:700;text-transform:uppercase;letter-spacing:0.1em">{hoveredNode.type}</span>
          <span style="font-size:14px;font-weight:600;color:var(--text-primary)">{hoveredNode.name}</span>
          <span style="font-size:11px;color:var(--text-muted)">{hoveredNode.edges} connections</span>
        </div>
      {/if}
    </div>

    <div class="legend">
      {#each Object.entries(typeColors) as [type, color]}
        <div class="legend-item">
          <div class="dot" style="background:{color}"></div>
          <span>{type}</span>
        </div>
      {/each}
    </div>

    <div class="breakdown-grid">
      {#if stats?.nodesByType}
        <div class="breakdown-card">
          <div class="bh"><div class="gold-line" style="margin-bottom:0"></div><span class="bt">NODES BY TYPE</span></div>
          <table>
            <thead><tr><th>TYPE</th><th style="text-align:right">COUNT</th></tr></thead>
            <tbody>
              {#each Object.entries(stats.nodesByType).sort((a,b) => Number(b[1]) - Number(a[1])) as [type, count]}
                <tr><td><span class="dot-inline" style="background:{getColor(type)}"></span>{type}</td><td class="mono" style="text-align:right">{count}</td></tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
      {#if stats?.edgesByType}
        <div class="breakdown-card">
          <div class="bh"><div class="gold-line" style="margin-bottom:0"></div><span class="bt">EDGES BY TYPE</span></div>
          <table>
            <thead><tr><th>TYPE</th><th style="text-align:right">COUNT</th></tr></thead>
            <tbody>
              {#each Object.entries(stats.edgesByType).sort((a,b) => Number(b[1]) - Number(a[1])) as [type, count]}
                <tr><td>{type}</td><td class="mono" style="text-align:right">{count}</td></tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .graph-page { display:flex; flex-direction:column; }
  .graph-header { display:flex; justify-content:space-between; align-items:flex-start; margin-bottom:14px; }
  .graph-header h3 { font-family:var(--font-display); font-size:16px; font-weight:600; }
  .graph-controls { display:flex; align-items:center; gap:12px; }
  .stat-mini { font-size:11px; color:var(--text-muted); }
  .btn-icon { padding:6px 10px; font-size:16px; }

  .canvas-wrapper {
    position:relative;
    background:#030303;
    border:1px solid var(--border);
    margin-bottom:14px;
    height: 480px;
    min-height: 320px;
    transition: height 0.2s;
  }
  .canvas-wrapper:hover { border-color:var(--accent); }
  canvas { display:block; width:100%; height:100%; cursor:grab; }
  canvas:active { cursor:grabbing; }

  /* Fullscreen overlay */
  .is-fullscreen {
    position: fixed;
    inset: 0;
    z-index: 1000;
    background: #030303;
    padding: 16px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
  }
  .is-fullscreen .canvas-wrapper {
    height: calc(100vh - 120px);
    flex: 1;
  }

  .canvas-shell { width:100%; height:480px; min-height:320px; background:var(--bg-card); border:1px solid var(--border); animation:pulse 1.5s ease-in-out infinite; }
  @keyframes pulse { 0%,100%{opacity:.3} 50%{opacity:.6} }

  .tooltip { position:absolute; top:12px; left:12px; background:rgba(3,3,3,.9); border:1px solid var(--accent); padding:10px 16px; display:flex; flex-direction:column; gap:3px; pointer-events:none; }

  .legend { display:flex; flex-wrap:wrap; gap:14px; margin-bottom:14px; padding:10px 14px; background:var(--bg-card); border:1px solid var(--border); }
  .legend-item { display:flex; align-items:center; gap:6px; font-size:10px; color:var(--text-muted); font-family:var(--font-ui); text-transform:uppercase; letter-spacing:.06em; }
  .dot { width:8px; height:8px; border-radius:50%; flex-shrink:0; }
  .dot-inline { display:inline-block; width:6px; height:6px; border-radius:50%; margin-right:6px; }

  .breakdown-grid { display:grid; grid-template-columns:1fr 1fr; gap:16px; }
  @media(max-width:700px){.breakdown-grid{grid-template-columns:1fr}}
  .breakdown-card { background:var(--bg-card); border:1px solid var(--border); padding:18px; }
  .breakdown-card:hover { border-color:var(--accent); box-shadow:var(--shadow-hover); }
  .bh { display:flex; align-items:center; gap:10px; margin-bottom:10px; }
  .bt { font-family:var(--font-ui); font-size:10px; font-weight:700; color:var(--accent); text-transform:uppercase; letter-spacing:.12em; }
</style>
