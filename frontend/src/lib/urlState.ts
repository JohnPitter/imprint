// Helpers minimalistas pra refletir estado de UI no query string da URL.
// Sem framework router — só window.history. Permite refresh-resistant
// e shareable links pra estados como ?days=7&tags=auth,decay.
//
// Convenção: chaves com valor vazio/zero/false são removidas do URL pra
// manter o link curto. Lê via getURLState() na inicialização.

export function getURLState(): Record<string, string> {
  if (typeof window === 'undefined') return {};
  const out: Record<string, string> = {};
  const params = new URLSearchParams(window.location.search);
  params.forEach((value, key) => { out[key] = value; });
  return out;
}

// Atualiza o URL sem disparar navegação. Valores vazios/0/false somem.
// Usa replaceState pra não poluir o histórico do browser com cada toque
// no slider — UX padrão pra controles contínuos.
export function setURLState(updates: Record<string, string | number | boolean | null | undefined>) {
  if (typeof window === 'undefined') return;
  const params = new URLSearchParams(window.location.search);
  for (const [key, value] of Object.entries(updates)) {
    if (value === null || value === undefined || value === '' || value === false || value === 0) {
      params.delete(key);
    } else {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const newURL = window.location.pathname + (qs ? `?${qs}` : '') + window.location.hash;
  window.history.replaceState({}, '', newURL);
}
