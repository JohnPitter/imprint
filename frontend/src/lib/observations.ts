// Shared helpers for components that render observations (Sessions, Timeline).
// Both panels accept the same observation shape, so the field accessor, the
// LLM-bullet stripper, and the type taxonomies live here instead of being
// duplicated.

export const typeLabels: Record<string, string> = {
  file_operation: 'FILE',
  command_execution: 'CMD',
  search: 'SEARCH',
  error: 'ERROR',
  decision: 'DECISION',
  discovery: 'DISCOVERY',
  conversation: 'CONV',
  notification: 'NOTIFY',
  subagent_event: 'AGENT',
  task: 'TASK',
  other: 'OTHER',
};

export const typeColors: Record<string, string> = {
  file_operation: 'badge-info',
  command_execution: 'badge-accent',
  error: 'badge-danger',
  decision: 'badge-warning',
  discovery: 'badge-success',
  search: 'badge-info',
  conversation: 'badge-purple',
  notification: 'badge-warning',
  subagent_event: 'badge-accent',
  task: 'badge-success',
  other: 'badge-info',
};

// Pull a value from an object trying each candidate key in order. Observations
// come from the API in either Go-struct casing (Title) or JSON casing (title),
// so callers list both.
export function getField(o: Record<string, any>, ...keys: string[]): any {
  for (const k of keys) {
    if (o[k] !== undefined && o[k] !== null && o[k] !== '') return o[k];
  }
  return undefined;
}

// LLM responses sometimes leak unicode bullet artefacts and literal \uXXXX
// escape sequences into observation text. clean() strips both so the cards
// stay readable.
export function clean(s: string | undefined | null): string {
  if (!s) return '';
  return s
    .replace(/^[\s -⯿-¿•·–—►▸▶→←↑↓\-]+/, '')
    .replace(/\\?u[0-9a-fA-F]{4}\s?/g, '')
    .replace(/[•‣–—←→↓↔■-◿☀-⛿]+\s?/g, '')
    .trim();
}
