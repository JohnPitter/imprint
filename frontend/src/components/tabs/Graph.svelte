<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api } from '../../lib/api';
  import { createPoller } from '../../lib/poller';
  import ConceptTag from '../ConceptTag.svelte';

  let canvasEl: HTMLCanvasElement = $state(undefined as any);
  let stats: any = $state(null);
  let loading = $state(true);
  let nodeCount = $state(0);
  let edgeCount = $state(0);
  let hoveredNode: any = $state(null);
  let animFrame: number;

  // Hover ripple: ao entrar num node, marca timestamp. O draw() usa pra
  // animar um anel expansivo + fade durante RIPPLE_MS milissegundos.
  let hoverRippleAt = 0;
  const RIPPLE_MS = 700;

  // Synaptic pulses: partículas viajando ao longo das edges (visual de
  // "neurônio firing"). Cada pulse atravessa a edge em PULSE_DURATION_MS,
  // depois é removido. Pool de array é mutado in-place pra evitar GC.
  interface Pulse { eIdx: number; t0: number; reverse: boolean; color: string; }
  let pulses: Pulse[] = [];
  const PULSE_DURATION_MS = 800;
  const AMBIENT_PULSE_INTERVAL_MS = 250; // ~4 pulsos/s em idle
  let lastAmbientPulseAt = 0;

  // Cluster summary: cache local + debounce de 600ms no hover. Evita
  // disparar Haiku na primeira fração de segundo que o cursor passa
  // por cima — só pra hovers intencionais.
  let clusterSummaries: Record<number, string> = $state({});
  let clusterSummaryLoading = $state(false);
  let clusterSummaryDebounce: ReturnType<typeof setTimeout> | null = null;
  let hoveredCluster = $state(-1);

  // Calcula o cluster sob hover e dispara fetch debounced.
  function trackClusterHover(node: any) {
    const c = node ? node.community : -1;
    if (c === hoveredCluster) return;
    hoveredCluster = c;
    if (clusterSummaryDebounce) { clearTimeout(clusterSummaryDebounce); clusterSummaryDebounce = null; }
    if (c < 0 || clusterSummaries[c]) return;
    clusterSummaryDebounce = setTimeout(() => fetchClusterSummary(c), 600);
  }
  async function fetchClusterSummary(c: number) {
    if (clusterSummaries[c]) return;
    const ids = nodes.filter(n => n.community === c).map(n => n.id);
    if (ids.length < 2) return;
    clusterSummaryLoading = true;
    try {
      const r = await api.clusterSummary(ids) as any;
      if (r?.summary) clusterSummaries = { ...clusterSummaries, [c]: r.summary };
    } catch (e) {
      // 503 (sem LLM) é ok — fica sem summary, sem alarme.
    }
    clusterSummaryLoading = false;
  }

  // Memory graph view: top-N latest memories with edges between those
  // that share concepts. The legacy "concepts" view depended on backend
  // entity extraction that ran only on session-end and was removed for
  // simplicity.

  // Simulation space cresce proporcional a sqrt(N) pra manter densidade
  // visual constante (ver buildSimulation).
  let SIM_W = $state(1600);
  let SIM_H = $state(1200);

  interface SimNode {
    id: string; type: string; name: string;
    x: number; y: number; vx: number; vy: number;
    edges: number; radius: number;
    community: number;  // label propagation result; índice do cluster
    concepts: string[];
  }
  interface SimEdge { source: number; target: number; type: string; }

  // Paleta de cores por community. Saturação intermediária pra funcionar
  // como halo translúcido no light e dark theme.
  const communityPalette = [
    '#e8a065', '#5ba3d9', '#4ecdc4', '#a78bfa',
    '#f472b6', '#34d399', '#eab308', '#38bdf8',
    '#8b5cf6', '#fb923c', '#22c55e', '#ef4444',
  ];

  // Label Propagation: cada node começa em sua própria comunidade,
  // depois adota a comunidade mais frequente entre vizinhos. Em ~5 iters
  // converge pra grafos pequenos. O(N×degree×iters), sem alocação no loop.
  function detectCommunities(simNodes: SimNode[], simEdges: SimEdge[], iters = 5) {
    const n = simNodes.length;
    if (n === 0) return;
    // Adjacency list compacta
    const adj: number[][] = Array.from({ length: n }, () => []);
    for (const e of simEdges) {
      adj[e.source].push(e.target);
      adj[e.target].push(e.source);
    }
    // Init: cada node = sua própria comunidade
    for (let i = 0; i < n; i++) simNodes[i].community = i;
    // Itera ordem aleatória pra evitar bias topológico
    const order = Array.from({ length: n }, (_, i) => i);
    for (let it = 0; it < iters; it++) {
      // Fisher-Yates shuffle
      for (let i = n - 1; i > 0; i--) {
        const j = Math.floor(Math.random() * (i + 1));
        [order[i], order[j]] = [order[j], order[i]];
      }
      let changed = false;
      for (const i of order) {
        const neighbors = adj[i];
        if (neighbors.length === 0) continue;
        // Vota: quem é a comunidade mais comum entre vizinhos?
        const counts = new Map<number, number>();
        for (const j of neighbors) {
          const c = simNodes[j].community;
          counts.set(c, (counts.get(c) || 0) + 1);
        }
        // Pega o max — empate quebra escolhendo o menor índice (estável)
        let best = simNodes[i].community;
        let bestCount = -1;
        for (const [c, cnt] of counts) {
          if (cnt > bestCount || (cnt === bestCount && c < best)) {
            best = c; bestCount = cnt;
          }
        }
        if (best !== simNodes[i].community) {
          simNodes[i].community = best;
          changed = true;
        }
      }
      if (!changed) break; // convergiu
    }
    // Re-densifica IDs de community: 0..K-1 pra mapear na paleta
    const remap = new Map<number, number>();
    for (const node of simNodes) {
      if (!remap.has(node.community)) remap.set(node.community, remap.size);
      node.community = remap.get(node.community)!;
    }
  }

  let nodes: SimNode[] = $state([]);
  let edges: SimEdge[] = $state([]);
  let zoom = 1;
  let panX = 0, panY = 0;
  let dragging = false;
  let dragStartX = 0, dragStartY = 0;
  let panStartX = 0, panStartY = 0;
  let stopPoll: (() => void) | undefined;
  // Estado da câmera, escrito por draw() e lido por getSimCoords()
  // pra hit-test usar exatamente a mesma transform do desenho.
  let cam: { fitScale: number; bboxCx: number; bboxCy: number } = { fitScale: 1, bboxCx: 0, bboxCy: 0 };

  const typeColors: Record<string, string> = {
    // Knowledge graph types
    concept: '#e8a065', file: '#5ba3d9', function: '#4ecdc4', error: '#ef4444',
    decision: '#eab308', pattern: '#a78bfa', library: '#f472b6', person: '#34d399',
    project: '#c8933a', component: '#38bdf8', process: '#8b5cf6',
    // Memory types — different palette so the two views are visually distinct
    architecture: '#c8933a', preference: '#a78bfa', bug: '#ef4444',
    workflow: '#34d399', fact: '#eab308',
  };
  function getColor(type: string): string { return typeColors[type] || '#555'; }

  onMount(async () => {
    await refresh(true);
    // Poll every 10s; only rebuild the simulation when node/edge counts actually change.
    stopPoll = createPoller(() => refresh(false), 10000);
  });

  onDestroy(() => {
    if (animFrame) cancelAnimationFrame(animFrame);
    if (pendingDrawFrame) cancelAnimationFrame(pendingDrawFrame);
    stopPoll?.();
  });

  async function refresh(initial: boolean) {
    try {
      const g = await api.memoryGraph(200, 1) as any;
      const newNodeCount = g?.nodes?.length || 0;
      const newEdgeCount = g?.edges?.length || 0;
      const structureChanged = newNodeCount !== nodeCount || newEdgeCount !== edgeCount;
      const byType: Record<string, number> = {};
      for (const n of (g?.nodes || [])) byType[n.type || 'other'] = (byType[n.type || 'other'] || 0) + 1;
      stats = {
        totalNodes: newNodeCount,
        totalEdges: newEdgeCount,
        nodesByType: byType,
      };
      nodeCount = newNodeCount;
      edgeCount = newEdgeCount;
      if ((initial || structureChanged) && newNodeCount > 0) {
        buildSimulation(g.nodes, g.edges || []);
        startSimulation();
      }
    } catch (e) { console.error(e); }
    if (initial) loading = false;
  }

  function buildSimulation(rawNodes: any[], rawEdges: any[]) {
    const nodeMap = new Map<string, number>();

    // Memory edges trazem source/target + um peso numérico opcional.
    const edgeCounts = new Map<string, number>();
    for (const e of rawEdges) {
      const s = e.source || e.Source || '';
      const t = e.target || e.Target || '';
      edgeCounts.set(s, (edgeCounts.get(s) || 0) + 1);
      edgeCounts.set(t, (edgeCounts.get(t) || 0) + 1);
    }

    // Sim space cresce com sqrt(N) pra manter a densidade visual constante
    // entre 20 nodes e 200+. Sem isso o grafo apertava num bloco denso.
    const N = Math.max(1, rawNodes.length);
    const sqrtN = Math.sqrt(N);
    SIM_W = Math.max(1200, Math.round(140 * sqrtN));
    SIM_H = Math.max(900, Math.round(105 * sqrtN));

    // Distribuição inicial: spread radial aleatório (não círculo perfeito),
    // ocupando ~70% do raio do sim space pra a física ter espaço de trabalho.
    const cx = SIM_W / 2, cy = SIM_H / 2;
    const spread = Math.min(SIM_W, SIM_H) * 0.35;
    nodes = rawNodes.map((n: any, i: number) => {
      const id = n.id || n.ID || '';
      nodeMap.set(id, i);
      const ec = edgeCounts.get(id) || 0;
      const label = n.title || n.Title || n.name || n.Name || id;
      const strength = n.strength ?? n.Strength ?? 0;
      const baseRadius = Math.max(4, Math.min(20, 4 + ec * 0.6 + strength * 0.8));
      // Polar com r aleatório uniforme em área (sqrt) e ângulo livre.
      const angle = Math.random() * Math.PI * 2;
      const rr = spread * Math.sqrt(Math.random());
      // Concepts vêm como string JSON ou array — normaliza pra string[].
      const rawConcepts = n.concepts ?? n.Concepts ?? [];
      let conceptList: string[] = [];
      if (Array.isArray(rawConcepts)) {
        conceptList = rawConcepts.filter((s: any) => typeof s === 'string');
      } else if (typeof rawConcepts === 'string') {
        try { const p = JSON.parse(rawConcepts); if (Array.isArray(p)) conceptList = p; } catch { /* ignore */ }
      }
      return {
        id, type: n.type || n.Type || 'other', name: label,
        x: cx + Math.cos(angle) * rr,
        y: cy + Math.sin(angle) * rr,
        vx: (Math.random() - 0.5) * 0.5,
        vy: (Math.random() - 0.5) * 0.5,
        edges: ec,
        radius: baseRadius,
        community: 0,
        concepts: conceptList,
      };
    });

    edges = [];
    for (const e of rawEdges) {
      const si = nodeMap.get(e.source || e.Source || '');
      const ti = nodeMap.get(e.target || e.Target || '');
      if (si !== undefined && ti !== undefined) {
        edges.push({
          source: si,
          target: ti,
          type: `weight:${e.weight ?? 1}`,
        });
      }
    }

    // Detecta comunidades (label propagation, ~5 iters) — barato pra
    // 200 nodes/220 edges, roda 1× por buildSimulation. O resultado
    // colore o glow do node no draw().
    detectCommunities(nodes, edges);
  }

  // Múltiplos sim-steps por frame de RAF: comprime a convergência sem
  // bloquear o thread principal. STEPS_PER_FRAME=4 entrega ~480 iters em
  // ~2s a 60fps, com a UI 100% responsiva entre frames.
  const STEPS_PER_FRAME = 4;
  const MAX_ITERS = 480;
  const WARMUP_ITERS = 160;

  // Um único passo da simulação. Não desenha nem agenda RAF — o caller
  // decide se roda em laço síncrono (pre-warm) ou em RAF (polish).
  function simStep(iter: number, params: { repBase: number; att: number; grav: number; totalIters: number }): number {
    const n = nodes.length;
    // Simulated annealing: damp varia de 0.96 (warm) até 0.86 (cool).
    // Warm permite que nodes voem pra longe e descubram o layout global;
    // cool segura tudo no fim pra estabilizar sem oscilação.
    const t = Math.min(1, iter / params.totalIters);
    const damp = 0.96 - 0.10 * t;
    const cx = SIM_W / 2, cy = SIM_H / 2;

    for (let i = 0; i < n; i++) {
      const a = nodes[i];
      for (let j = i + 1; j < n; j++) {
        const b = nodes[j];
        const dx = b.x - a.x, dy = b.y - a.y;
        const d2 = dx*dx + dy*dy;
        if (d2 < 1) continue;
        const d = Math.sqrt(d2);
        const f = params.repBase / d2;
        const fx = (dx/d)*f, fy = (dy/d)*f;
        a.vx -= fx; a.vy -= fy;
        b.vx += fx; b.vy += fy;
      }
    }
    for (const e of edges) {
      const s = nodes[e.source], t2 = nodes[e.target];
      const dx = t2.x - s.x, dy = t2.y - s.y;
      const d = Math.max(1, Math.sqrt(dx*dx + dy*dy));
      const f = d * params.att;
      const fx = (dx/d)*f, fy = (dy/d)*f;
      s.vx += fx; s.vy += fy; t2.vx -= fx; t2.vy -= fy;
    }
    let energy = 0;
    for (const node of nodes) {
      node.vx += (cx - node.x) * params.grav;
      node.vy += (cy - node.y) * params.grav;
      node.vx *= damp; node.vy *= damp;
      node.x += node.vx; node.y += node.vy;
      energy += node.vx * node.vx + node.vy * node.vy;
    }
    return energy;
  }

  function startSimulation() {
    const n = nodes.length;
    // Repulsão cresce com N pra manter espaçamento médio entre vizinhos
    // proporcional ao tamanho do canvas. Atração e gravidade são
    // calibradas pra deixar a estrutura orgânica sem colapsar no centro.
    const params = {
      repBase: 14000 + n * 30,
      att: 0.0018,
      grav: 0.0006,
      totalIters: MAX_ITERS,
    };
    const energyFloor = Math.max(0.04, 0.4 / Math.sqrt(Math.max(1, n)));

    let iter = 0;
    function tick() {
      // Roda múltiplos passos de física por frame mas só desenha 1×.
      // Cada frame ainda libera o thread entre RAF callbacks, então a UI
      // continua responsiva (scroll, hover, cliques em outras abas).
      let energy = 0;
      const remaining = MAX_ITERS - iter;
      const steps = Math.min(STEPS_PER_FRAME, remaining);
      for (let k = 0; k < steps; k++) {
        iter++;
        energy = simStep(iter, params);
      }
      draw();
      const settled = iter > WARMUP_ITERS && energy / Math.max(1, n) < energyFloor;
      if (!settled && iter < MAX_ITERS) {
        animFrame = requestAnimationFrame(tick);
      } else {
        // Convergiu: transitiona pra idle animation perpétua.
        // O grafo nunca "morre" — fica respirando enquanto visível.
        startIdleAnimation();
      }
    }
    animFrame = requestAnimationFrame(tick);
  }

  // Idle animation: rodando indefinidamente após convergência da física.
  // Adiciona ruído mínimo nas velocidades + damp alto pra criar a sensação
  // de "respiração" — nodes flutuam suavemente sem nunca perderem a
  // posição relativa. O draw() ainda anima pulse + ripple via timestamp.
  // Suspende automaticamente quando a aba do browser fica invisível pra
  // não queimar CPU à toa.
  function startIdleAnimation() {
    if (animFrame) cancelAnimationFrame(animFrame);
    let lastFrameTs = 0;
    function idleTick(ts: number) {
      if (document.visibilityState !== 'visible') {
        // Aba escondida: pausa, RAF continua acordando mas não trabalha.
        animFrame = requestAnimationFrame(idleTick);
        return;
      }
      // Throttle a 30fps — animações sutis não precisam de 60. Reduz CPU
      // pela metade e visualmente é indistinguível pra esse tipo de efeito.
      if (ts - lastFrameTs < 33) {
        animFrame = requestAnimationFrame(idleTick);
        return;
      }
      lastFrameTs = ts;
      idleStep();
      // Spawn ambient pulses em ritmo controlado. Throttled por
      // AMBIENT_PULSE_INTERVAL_MS pra não acumular se framerate variar.
      if (ts - lastAmbientPulseAt >= AMBIENT_PULSE_INTERVAL_MS) {
        spawnAmbientPulse(ts);
        lastAmbientPulseAt = ts;
      }
      reapPulses(ts);
      draw();
      animFrame = requestAnimationFrame(idleTick);
    }
    animFrame = requestAnimationFrame(idleTick);
  }

  // idleStep: O(N), barato. Cada node ganha um empurrão aleatório
  // pequeno; damp 0.96 faz a energia decair sem nunca chegar a zero.
  // Resultado visual: flutuação orgânica perpétua.
  function idleStep() {
    const noise = 0.04;
    const damp = 0.96;
    for (const node of nodes) {
      node.vx += (Math.random() - 0.5) * noise;
      node.vy += (Math.random() - 0.5) * noise;
      node.vx *= damp;
      node.vy *= damp;
      node.x += node.vx;
      node.y += node.vy;
    }
  }

  // spawnAmbientPulse: escolhe uma edge aleatória e dispara um pulso.
  // Direção também aleatória (50/50 source→target ou reverse) pra simular
  // tráfego bidirecional típico de rede neural.
  function spawnAmbientPulse(now: number) {
    if (edges.length === 0) return;
    const eIdx = Math.floor(Math.random() * edges.length);
    const e = edges[eIdx];
    const source = nodes[e.source];
    if (!source) return;
    const color = communityPalette[source.community % communityPalette.length];
    pulses.push({
      eIdx,
      t0: now,
      reverse: Math.random() < 0.5,
      color,
    });
  }

  // spawnHoverBurst: dispara 1 pulso em cada edge adjacente ao node hovered.
  // Cap em 8 pra nodes super conectados não saturarem visualmente. Pulsos
  // sempre saem DO hovered (não entram nele) — feedback visual de "este
  // node está ativando seus vizinhos".
  function spawnHoverBurst(node: any, now: number) {
    const nodeIdx = nodes.indexOf(node);
    if (nodeIdx < 0) return;
    const color = communityPalette[node.community % communityPalette.length];
    let spawned = 0;
    for (let i = 0; i < edges.length && spawned < 8; i++) {
      const e = edges[i];
      if (e.source === nodeIdx) {
        pulses.push({ eIdx: i, t0: now, reverse: false, color });
        spawned++;
      } else if (e.target === nodeIdx) {
        // edge entra no hovered: reverse pra sair dele
        pulses.push({ eIdx: i, t0: now, reverse: true, color });
        spawned++;
      }
    }
  }

  // Remove pulsos vencidos. Filter cria array novo (pequeno overhead) mas
  // simplifica o código vs splice in-place. Pulsos vivos são poucos
  // (tipicamente <10), custo desprezível.
  function reapPulses(now: number) {
    pulses = pulses.filter(p => now - p.t0 < PULSE_DURATION_MS);
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

    // Timestamp único pro frame — usado em pulse + ripple. Performance.now
    // é monotonic e barato, sem risco de pulos como Date.now em sleep.
    const tNow = performance.now();

    // Canvas API ignora CSS vars; lê o token de tema na hora do draw
    // pra labels seguirem light/dark sem listener manual.
    const labelColor = getComputedStyle(document.documentElement)
      .getPropertyValue('--text-primary').trim() || '#f4f4f5';

    // Câmera: fit-to-bounds usando percentil 5–95, não min/max absoluto.
    // Outliers (1-2 nodes desconectados que voaram pra longe) puxavam o
    // fitScale pra um valor minúsculo e o cluster principal aparecia
    // como uma bolinha no centro. Percentil ignora os extremos —
    // outliers continuam visíveis, mas não dominam o framing.
    const xs: number[] = []; const ys: number[] = [];
    for (const n of nodes) { xs.push(n.x); ys.push(n.y); }
    xs.sort((a, b) => a - b); ys.sort((a, b) => a - b);
    const pct = (arr: number[], p: number) => {
      if (arr.length === 0) return 0;
      const i = Math.floor(arr.length * p);
      return arr[Math.min(arr.length - 1, Math.max(0, i))];
    };
    let minX: number, minY: number, maxX: number, maxY: number;
    if (xs.length === 0) {
      minX = 0; minY = 0; maxX = SIM_W; maxY = SIM_H;
    } else {
      minX = pct(xs, 0.05); maxX = pct(xs, 0.95);
      minY = pct(ys, 0.05); maxY = pct(ys, 0.95);
    }
    const pad = 80;
    const bboxW = Math.max(1, maxX - minX) + pad * 2;
    const bboxH = Math.max(1, maxY - minY) + pad * 2;
    const fitScale = Math.min(CW / bboxW, CH / bboxH);
    const bboxCx = (minX + maxX) / 2;
    const bboxCy = (minY + maxY) / 2;
    // Persiste a câmera atual pra getSimCoords inverter exatamente esta
    // mesma transform — single source of truth pra hover/click hit-test.
    cam = { fitScale, bboxCx, bboxCy };

    ctx.save();
    ctx.translate(panX + CW / 2, panY + CH / 2);
    ctx.scale(zoom * fitScale, zoom * fitScale);
    ctx.translate(-bboxCx, -bboxCy);

    // Edges — always visible. Opacidade calibrada pra contrastar
    // tanto no fundo escuro quanto no light theme.
    ctx.lineWidth = 0.9;
    ctx.strokeStyle = 'rgba(200,147,58,0.55)';
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
      ctx.strokeStyle = 'rgba(200,147,58,0.85)';
      ctx.beginPath();
      for (const e of edges) {
        if (e.source === hi || e.target === hi) {
          ctx.moveTo(nodes[e.source].x, nodes[e.source].y);
          ctx.lineTo(nodes[e.target].x, nodes[e.target].y);
        }
      }
      ctx.stroke();
    }

    // Synaptic pulses — partículas viajando ao longo das edges como
    // sinapses firing. Desenhados aqui (depois das edges, antes dos
    // nodes) pra ficarem sobre as linhas mas atrás dos núcleos.
    // Cada pulse: lerp posição source→target conforme tempo decorrido.
    // Visual: disc com glow radial; opacidade decai no fim pra fade-out.
    //
    // Tamanho compensa o zoom: em zoom baixo (fitScale aplicado encolhe
    // tudo), pulse cresce no sim space pra continuar visível em px.
    // Cap em 0.4 evita que em zoom muito longe o pulse fique gigante.
    const effScale = zoom * fitScale;
    const sizeBoost = 1 / Math.max(0.4, effScale);
    const glowR = 16 * sizeBoost;
    const coreR = 4 * sizeBoost;
    for (const p of pulses) {
      const e = edges[p.eIdx];
      if (!e) continue;
      const s = nodes[p.reverse ? e.target : e.source];
      const t = nodes[p.reverse ? e.source : e.target];
      if (!s || !t) continue;
      const progress = (tNow - p.t0) / PULSE_DURATION_MS;
      if (progress < 0 || progress > 1) continue;
      const px = s.x + (t.x - s.x) * progress;
      const py = s.y + (t.y - s.y) * progress;
      // Opacidade: brilha forte no meio do trajeto, fade nas pontas.
      // Curva sin(πt) dá esse "swell" característico de pulso elétrico.
      const alpha = Math.sin(progress * Math.PI);
      // Glow externo (suave)
      const grad = ctx.createRadialGradient(px, py, 0, px, py, glowR);
      grad.addColorStop(0, p.color + 'ee');
      grad.addColorStop(0.35, p.color + '77');
      grad.addColorStop(1, p.color + '00');
      ctx.fillStyle = grad;
      ctx.globalAlpha = alpha;
      ctx.beginPath(); ctx.arc(px, py, glowR, 0, Math.PI * 2); ctx.fill();
      // Core brilhante no centro
      ctx.fillStyle = p.color;
      ctx.beginPath(); ctx.arc(px, py, coreR, 0, Math.PI * 2); ctx.fill();
      ctx.globalAlpha = 1;
    }

    // Nodes — core mantém cor do TIPO, glow ganha cor da COMUNIDADE.
    // Resultado: o tipo continua identificável de relance, mas o halo
    // revela "ilhas de conhecimento" relacionadas (cluster topológico).
    // Pulse: glow de nodes "importantes" (radius>=10) modula em sin com
    // período de ~3s. Ondas começam dessincronizadas (offset por hash do
    // id) pra parecer respiração natural, não strobing sincronizado.
    for (const n of nodes) {
      const color = getColor(n.type);
      const communityColor = communityPalette[n.community % communityPalette.length];
      const hov = n === hoveredNode;
      // Pulse fator: 1.0 base, oscila ±0.25 nos importantes. Hash simples
      // pelo charCode do primeiro id char dessincroniza nodes vizinhos.
      const pulsePhase = (n.id.charCodeAt(0) || 0) * 0.3;
      const pulse = n.radius >= 10
        ? 1 + 0.25 * Math.sin(tNow / 1500 + pulsePhase)
        : 1;
      // Glow tinto pela community
      if (n.radius > 5 || hov) {
        const glowR = n.radius * (hov ? 4 : 2.5) * pulse;
        const g = ctx.createRadialGradient(n.x, n.y, 0, n.x, n.y, glowR);
        g.addColorStop(0, communityColor + (hov ? '50' : '20'));
        g.addColorStop(1, communityColor + '00');
        ctx.fillStyle = g;
        ctx.beginPath(); ctx.arc(n.x, n.y, glowR, 0, Math.PI*2); ctx.fill();
      }
      // Core
      ctx.fillStyle = color;
      ctx.globalAlpha = hov ? 1 : 0.88;
      ctx.beginPath(); ctx.arc(n.x, n.y, n.radius, 0, Math.PI*2); ctx.fill();
      ctx.globalAlpha = 1;
      // Ripple expansivo no node em hover. Cresce de radius pra radius*5
      // ao longo de RIPPLE_MS, opacidade decai linearmente.
      if (hov && hoverRippleAt > 0) {
        const elapsed = tNow - hoverRippleAt;
        if (elapsed >= 0 && elapsed < RIPPLE_MS) {
          const t = elapsed / RIPPLE_MS;
          const ringR = n.radius * (1 + t * 4);
          ctx.strokeStyle = communityColor;
          ctx.globalAlpha = (1 - t) * 0.7;
          ctx.lineWidth = 1.5;
          ctx.beginPath(); ctx.arc(n.x, n.y, ringR, 0, Math.PI*2); ctx.stroke();
          ctx.globalAlpha = 1;
        }
      }
      // Label
      if (n.radius > 7 || hov) {
        ctx.fillStyle = labelColor;
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
    // Inverte a transform aplicada em draw():
    //   ctx.translate(panX + CW/2, panY + CH/2)
    //   ctx.scale(zoom * fitScale)
    //   ctx.translate(-bboxCx, -bboxCy)
    const k = zoom * cam.fitScale;
    const sx = (clientX - rect.left - panX - CW/2) / k + cam.bboxCx;
    const sy = (clientY - rect.top  - panY - CH/2) / k + cam.bboxCy;
    return [sx, sy];
  }

  // Mouse moves fire ~60Hz; collapse them into a single draw per animation
  // frame so we never queue up redundant work.
  let pendingDrawFrame = 0;
  function scheduleDraw() {
    if (pendingDrawFrame) return;
    pendingDrawFrame = requestAnimationFrame(() => {
      pendingDrawFrame = 0;
      draw();
    });
  }

  function onMouseMove(e: MouseEvent) {
    if (dragging) {
      panX = panStartX + (e.clientX - dragStartX);
      panY = panStartY + (e.clientY - dragStartY);
      scheduleDraw();
      return;
    }
    const [mx, my] = getSimCoords(e.clientX, e.clientY);
    const prev = hoveredNode;
    hoveredNode = null;
    for (const n of nodes) {
      const dx = n.x - mx, dy = n.y - my;
      const r = n.radius + 6;
      if (dx*dx + dy*dy < r*r) { hoveredNode = n; break; }
    }
    if (canvasEl) canvasEl.style.cursor = hoveredNode ? 'pointer' : 'grab';
    // Only redraw if hover state actually changed.
    if (prev !== hoveredNode) {
      scheduleDraw();
      trackClusterHover(hoveredNode);
      // Dispara ripple só ao ENTRAR num node (transitar de null/outro
      // pra um novo). Sair pra null não dispara — evita ripple ao
      // afastar o cursor.
      if (hoveredNode && hoveredNode !== prev) {
        const now = performance.now();
        hoverRippleAt = now;
        // Burst sináptico nas edges adjacentes — rede "reage" ao toque.
        spawnHoverBurst(hoveredNode, now);
      }
    }
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
    scheduleDraw();
  }

  // Botões discretos de zoom: usam o mesmo step do scroll wheel pra
  // sensação consistente. Limites idênticos ao onWheel.
  function zoomIn()  { zoom = Math.min(8, zoom * 1.2);  scheduleDraw(); }
  function zoomOut() { zoom = Math.max(0.15, zoom * 0.8); scheduleDraw(); }
  function resetView() { zoom = 1; panX = 0; panY = 0; scheduleDraw(); }

  let fullscreen = $state(false);

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

<svelte:window onkeydown={onKeyDown} />

<div class="graph-page" class:is-fullscreen={fullscreen}>
  <div class="graph-header">
    <div>
      <div class="gold-line"></div>
      <h3>Knowledge Graph</h3>
    </div>
    <div class="graph-controls">
      <span class="stat-mini mono">{nodeCount} nodes · {edgeCount} edges</span>
      <button class="btn" onclick={resetView}>Reset</button>
      <button class="btn btn-icon" onclick={toggleFullscreen} title={fullscreen ? 'Exit fullscreen (Esc)' : 'Fullscreen'}>
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
        onmousemove={onMouseMove}
        onmousedown={onMouseDown}
        onmouseup={onMouseUp}
        onmouseleave={onMouseUp}
        onwheel={(e) => { e.preventDefault(); onWheel(e); }}
      ></canvas>

      <!-- Zoom controls flutuantes (canto inferior direito do canvas).
           Mais ergonômico que botões na header — fica perto do conteúdo
           que o user está olhando, sem competir com a tooltip. -->
      <div class="zoom-controls">
        <button class="zoom-btn" onclick={zoomIn} title="Zoom in (+)" aria-label="Zoom in">+</button>
        <button class="zoom-btn" onclick={zoomOut} title="Zoom out (−)" aria-label="Zoom out">−</button>
        <button class="zoom-btn" onclick={resetView} title="Reset view" aria-label="Reset view">⊙</button>
      </div>
      {#if hoveredNode}
        <div class="tooltip">
          <span style="color:{getColor(hoveredNode.type)};font-size:10px;font-weight:700;text-transform:uppercase;letter-spacing:0.1em">{hoveredNode.type}</span>
          <span style="font-size:14px;font-weight:600;color:var(--text-primary)">{hoveredNode.name}</span>
          <span style="font-size:11px;color:var(--text-muted)">{hoveredNode.edges} connections</span>
          {#if hoveredNode.concepts && hoveredNode.concepts.length > 0}
            <div class="tooltip-tags">
              {#each hoveredNode.concepts.slice(0, 8) as c}<ConceptTag label={c} />{/each}
            </div>
          {/if}
          {#if hoveredCluster >= 0 && clusterSummaries[hoveredCluster]}
            <div class="tooltip-cluster">
              <span class="tooltip-cluster-label">CLUSTER</span>
              <span class="tooltip-cluster-text">{clusterSummaries[hoveredCluster]}</span>
            </div>
          {:else if hoveredCluster >= 0 && clusterSummaryLoading}
            <div class="tooltip-cluster">
              <span class="tooltip-cluster-label">CLUSTER</span>
              <span class="tooltip-cluster-text" style="opacity:0.6">resumindo…</span>
            </div>
          {/if}
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
    background:var(--bg-card);
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
    background: var(--bg-primary);
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

  /* Zoom controls flutuantes — canto inferior direito do canvas. */
  .zoom-controls {
    position: absolute;
    bottom: 12px;
    right: 12px;
    display: flex;
    flex-direction: column;
    gap: 4px;
    z-index: 2;
  }
  .zoom-btn {
    width: 32px;
    height: 32px;
    background: var(--bg-card);
    border: 1px solid var(--border);
    color: var(--text-dim);
    font-size: 16px;
    font-family: var(--font-ui);
    line-height: 1;
    cursor: pointer;
    transition: all 0.15s var(--ease);
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .zoom-btn:hover {
    color: var(--accent);
    border-color: var(--accent);
    background: var(--bg-hover);
  }
  .zoom-btn:active { transform: translateY(1px); }

  .tooltip {
    position: absolute;
    top: 12px; left: 12px;
    background: var(--bg-card);
    border: 1px solid var(--accent);
    padding: 10px 16px;
    display: flex;
    flex-direction: column;
    gap: 6px;
    pointer-events: none;
    box-shadow: var(--shadow);
    max-width: 360px;
  }
  .tooltip-tags { display: flex; flex-wrap: wrap; gap: 4px; max-width: 320px; margin-top: 4px; }
  .tooltip-cluster {
    display: flex;
    flex-direction: column;
    gap: 2px;
    margin-top: 4px;
    padding-top: 6px;
    border-top: 1px solid var(--border);
  }
  .tooltip-cluster-label {
    font-family: var(--font-ui);
    font-size: 9px;
    font-weight: 700;
    color: var(--accent);
    letter-spacing: 0.12em;
  }
  .tooltip-cluster-text {
    font-size: 12px;
    color: var(--text-secondary);
    font-style: italic;
  }

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
