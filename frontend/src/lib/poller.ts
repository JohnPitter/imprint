// Shared polling helper. Consumers call `createPoller(refresh, 10000)` from
// onMount and use the returned cleanup in onDestroy. Centralising it here
// ensures every tab clears its interval on tab switch and keeps the
// boilerplate out of the components.
export function createPoller(fn: () => unknown | Promise<unknown>, intervalMs: number): () => void {
  const timer = setInterval(() => {
    try {
      const r = fn();
      if (r && typeof (r as Promise<unknown>).catch === 'function') {
        (r as Promise<unknown>).catch(() => { /* swallow — components log their own errors */ });
      }
    } catch { /* swallow */ }
  }, intervalMs);
  return () => clearInterval(timer);
}
