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
  listSessions: (limit = 50) => request<unknown>('GET', `/sessions?limit=${limit}`),
  startSession: (data: Record<string, unknown>) => request<unknown>('POST', '/session/start', data),
  endSession: (data: Record<string, unknown>) => request<unknown>('POST', '/session/end', data),

  // Observations
  listObservations: (sessionId: string) => request<unknown>('GET', `/observations?sessionId=${sessionId}`),

  // Memories
  listMemories: (type = '', limit = 50) => request<unknown>('GET', `/memories?type=${type}&limit=${limit}`),
  remember: (data: Record<string, unknown>) => request<unknown>('POST', '/remember', data),
  forget: (data: Record<string, unknown>) => request<unknown>('POST', '/forget', data),
  evolve: (data: Record<string, unknown>) => request<unknown>('POST', '/evolve', data),

  // Search
  search: (query: string, limit = 20) => request<unknown>('POST', '/search', { query, limit }),

  // Graph
  graphStats: () => request<unknown>('GET', '/graph/stats'),
  graphQuery: (startNodeId: string, maxDepth = 2) => request<unknown>('POST', '/graph/query', { startNodeId, maxDepth }),

  // Actions
  listActions: (status = '', limit = 50) => request<unknown>('GET', `/actions?status=${status}&limit=${limit}`),
  createAction: (data: Record<string, unknown>) => request<unknown>('POST', '/actions', data),
  frontier: () => request<unknown>('GET', '/frontier'),

  // Crystals
  listCrystals: (limit = 20) => request<unknown>('GET', `/crystals?limit=${limit}`),

  // Lessons
  listLessons: (limit = 50) => request<unknown>('GET', `/lessons?limit=${limit}`),
  searchLessons: (query: string) => request<unknown>('POST', '/lessons/search', { query }),

  // Insights
  listInsights: (limit = 50) => request<unknown>('GET', `/insights?limit=${limit}`),

  // Audit
  listAudit: (limit = 50, offset = 0) => request<unknown>('GET', `/audit?limit=${limit}&offset=${offset}`),

  // Summarize
  summarize: (data: Record<string, unknown>) => request<unknown>('POST', '/summarize', data),

  // Context
  buildContext: (data: Record<string, unknown>) => request<unknown>('POST', '/context', data),

  // Settings
  getSettings: () => request<unknown>('GET', '/settings'),
  updateSettings: (data: Record<string, unknown>) => request<unknown>('POST', '/settings', data),
};
