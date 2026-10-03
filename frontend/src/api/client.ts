import type {
  ApiErrorBody,
  CharacterTalents,
  DamageRequest,
  DamageResponse,
  EnemyPreset,
  GoodExport,
  ImportSummary,
  InsightsResponse,
  RosterEntry,
  SetInfo,
  SolveProgress,
  SolveRequest,
  WeaponOption,
} from './types';

export class ApiError extends Error {}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    headers: init?.body ? { 'Content-Type': 'application/json' } : undefined,
    ...init,
  });
  if (!res.ok) {
    let message = `${res.status} ${res.statusText}`;
    try {
      const body = (await res.json()) as ApiErrorBody;
      if (body.error) message = body.error;
    } catch {
      // ignore parse failure, use status text
    }
    throw new ApiError(message);
  }
  return res.json() as Promise<T>;
}

export const api = {
  fetchSample: () => request<GoodExport>('/api/sample'),
  importGood: (raw: string) =>
    request<ImportSummary>('/api/import', { method: 'POST', body: raw }),
  importExport: (data: GoodExport) =>
    request<ImportSummary>('/api/import', { method: 'POST', body: JSON.stringify(data) }),
  characters: (lang?: string) => request<RosterEntry[]>('/api/characters' + (lang ? `?lang=${lang}` : '')),
  sets: (lang?: string) => request<SetInfo[]>('/api/artifacts/sets' + (lang ? `?lang=${lang}` : '')),
  insights: (lang?: string) => request<InsightsResponse>('/api/insights' + (lang ? `?lang=${lang}` : '')),
  weapons: (type?: string, lang?: string) => {
    const params = new URLSearchParams();
    if (type) params.set('type', type);
    if (lang) params.set('lang', lang);
    const qs = params.toString();
    return request<WeaponOption[]>('/api/weapons' + (qs ? `?${qs}` : ''));
  },
  solveStart: (req: SolveRequest) =>
    request<{ jobId: string }>('/api/solve', { method: 'POST', body: JSON.stringify(req) }),
  solveProgress: (jobId: string) =>
    request<SolveProgress>(`/api/solve/${encodeURIComponent(jobId)}/progress`),
  characterTalents: (key: string, lang?: string) =>
    request<CharacterTalents>(`/api/characters/${encodeURIComponent(key)}/talents` + (lang ? `?lang=${lang}` : '')),
  enemies: (lang?: string) => request<EnemyPreset[]>('/api/enemies' + (lang ? `?lang=${lang}` : '')),
  calcDamage: (req: DamageRequest) =>
    request<DamageResponse>('/api/damage', { method: 'POST', body: JSON.stringify(req) }),
};
