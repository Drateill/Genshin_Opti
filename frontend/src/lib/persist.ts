// Persists the last import + Configure selections to localStorage so a
// page refresh (or the backend's in-memory store having nothing yet)
// doesn't force the user to re-upload/re-paste their GOOD export. The
// backend itself keeps no state across restarts by design (see
// internal/store/store.go) — this is what makes a refresh feel seamless
// anyway, by silently re-sending the same import.
import type { StatRange } from '../api/types';
import type { Accent } from './refdata';
import type { View } from '../components/TopBar';
import type { Lang } from '../i18n';

const KEY = 'artifact-optimizer:state';

export interface PersistedState {
  rawText?: string;
  view?: View;
  charKey?: string;
  targetSetKey?: string;
  targetSetKey2?: string;
  weaponId?: number;
  sands?: string[];
  goblet?: string[];
  circlet?: string[];
  constraints?: Record<string, StatRange>;
  objective?: string;
  topN?: number;
  includeEquippedByOthers?: boolean;
  team?: (string | null)[];
  accent?: Accent;
  showSolverStats?: boolean;
  defaultTopN?: number;
  lang?: Lang;
}

export function loadPersisted(): PersistedState {
  try {
    const raw = localStorage.getItem(KEY);
    return raw ? (JSON.parse(raw) as PersistedState) : {};
  } catch {
    return {};
  }
}

export function savePersisted(state: PersistedState) {
  try {
    localStorage.setItem(KEY, JSON.stringify(state));
  } catch {
    // Private browsing, quota exceeded, storage disabled — persistence is a
    // convenience, never something to crash the app over.
  }
}

export function clearPersisted() {
  try {
    localStorage.removeItem(KEY);
  } catch {
    // ignore
  }
}
