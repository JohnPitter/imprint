const BASE = '/imprint';

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const opts: RequestInit = {
    method,
    headers: { 'Content-Type': 'application/json' },
  };
  if (body) opts.body = JSON.stringify(body);
  const res = await fetch(BASE + path, opts);
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || res.statusText);
  }
  return res.json();
}

export const api = {
  // Health
  health: () => request<unknown>('GET', '/health'),

  // Sessions
  listSessions: (limit = 50, offset = 0) => request<unknown>('GET', `/sessions?limit=${limit}&offset=${offset}`),
  startSession: (data: Record<string, unknown>) => request<unknown>('POST', '/session/start', data),
  endSession: (data: Record<string, unknown>) => request<unknown>('POST', '/session/end', data),

  // Observations
  listObservations: (sessionId: string) => request<unknown>('GET', `/observations?sessionId=${sessionId}`),
  countObservations: () => request<unknown>('GET', `/observations/count`),

  // Memories
  listMemories: (type = '', limit = 50, offset = 0, before = '') => {
    const qs = new URLSearchParams({ type, limit: String(limit), offset: String(offset) });
    if (before) qs.set('before', before);
    return request<unknown>('GET', `/memories?${qs.toString()}`);
  },
  topConcepts: (limit = 20) => request<unknown>('GET', `/memories/concepts?limit=${limit}`),
  memoryHistory: (id: string) => request<unknown>('GET', `/memories/history?id=${encodeURIComponent(id)}`),
  memoryGraph: (topN = 200, minShared = 1) => request<unknown>('GET', `/memories/graph?topN=${topN}&minShared=${minShared}`),
  remember: (data: Record<string, unknown>) => request<unknown>('POST', '/remember', data),
  forget: (data: Record<string, unknown>) => request<unknown>('POST', '/forget', data),
  evolve: (data: Record<string, unknown>) => request<unknown>('POST', '/evolve', data),
  pinMemory: (id: string, pinned: boolean) => request<unknown>('POST', '/memories/pin', { id, pinned }),
  setMemoryConcepts: (id: string, concepts: string[]) => request<unknown>('POST', '/memories/concepts', { id, concepts }),
  clusterSummary: (ids: string[]) => request<unknown>('POST', '/memories/cluster-summary', { ids }),

  // Sessions
  sessionTimeline: (id: string, limit = 500) => request<unknown>('GET', `/sessions/timeline?id=${encodeURIComponent(id)}&limit=${limit}`),

  // Search
  search: (query: string, limit = 20) => request<unknown>('POST', '/search', { query, limit }),

  // Graph
  graphStats: () => request<unknown>('GET', '/graph/stats'),
  graphAll: () => request<unknown>('GET', '/graph/all'),
  graphQuery: (startNodeId: string, maxDepth = 2) => request<unknown>('POST', '/graph/query', { startNodeId, maxDepth }),

  // Actions
  listActions: (status = '', limit = 50, offset = 0) => request<unknown>('GET', `/actions?status=${status}&limit=${limit}&offset=${offset}`),
  createAction: (data: Record<string, unknown>) => request<unknown>('POST', '/actions', data),
  frontier: () => request<unknown>('GET', '/frontier'),

  // Lessons
  listLessons: (limit = 50, offset = 0) => request<unknown>('GET', `/lessons?limit=${limit}&offset=${offset}`),
  searchLessons: (query: string) => request<unknown>('POST', '/lessons/search', { query }),
  dismissLesson: (id: string) => request<unknown>('POST', '/lessons/dismiss', { id }),

  // Insights
  listInsights: (limit = 50, offset = 0) => request<unknown>('GET', `/insights?limit=${limit}&offset=${offset}`),

  // Audit
  listAudit: (limit = 50, offset = 0) => request<unknown>('GET', `/audit?limit=${limit}&offset=${offset}`),
  auditHeatmap: (days = 365) => request<unknown>('GET', `/audit/heatmap?days=${days}`),

  // Summarize
  summarize: (data: Record<string, unknown>) => request<unknown>('POST', '/summarize', data),

  // Context
  buildContext: (data: Record<string, unknown>) => request<unknown>('POST', '/context', data),

  // Settings
  getSettings: () => request<unknown>('GET', '/settings'),
  updateSettings: (data: Record<string, unknown>) => request<unknown>('POST', '/settings', data),

  // Pipeline status
  pipelineStatus: () => request<unknown>('GET', '/pipeline/status'),

  // Recall (search + LLM synthesis)
  recall: (query: string, limit = 8) => request<unknown>('POST', '/recall', { query, limit }),

  // Economy (Phase 1 token saldo meter)
  economy: (project = '', sinceDays = 0) => {
    const qs = new URLSearchParams();
    if (project) qs.set('project', project);
    if (sinceDays > 0) qs.set('sinceDays', String(sinceDays));
    const s = qs.toString();
    return request<unknown>('GET', `/economy${s ? `?${s}` : ''}`);
  },

  // Intuitions (Phase 2 rooted layer — inspection screen, invariant 11)
  listIntuitions: (project = '') =>
    request<unknown>('GET', `/intuitions${project ? `?project=${encodeURIComponent(project)}` : ''}`),
  intuitionContradictions: (id: string) =>
    request<unknown>('GET', `/intuitions/contradictions?id=${encodeURIComponent(id)}`),
  demoteIntuition: (id: string) => request<unknown>('POST', '/intuitions/demote', { id }),
  deleteIntuition: (id: string) => request<unknown>('POST', '/intuitions/delete', { id }),
  detectIntuitions: (project: string) => request<unknown>('POST', '/intuitions/detect', { project }),

  // Memory governance (A5)
  exportMemory: (project: string) =>
    request<unknown>('GET', `/memory/export?project=${encodeURIComponent(project)}`),
  purgeMemory: (project: string) => request<unknown>('POST', '/memory/purge', { project }),
  resetMemory: () => request<unknown>('POST', '/memory/reset', { confirm: true }),
};
