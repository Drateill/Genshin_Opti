import { chardb } from './refdata';

export function fmtStat(key: string, value: number, locale = 'en-US'): string {
  if (chardb.pctStats.has(key)) return `${value.toFixed(1)}%`;
  if (key === 'em') return String(Math.round(value));
  return Math.round(value).toLocaleString(locale);
}

export function fmtPct(value: number): string {
  return `${value.toFixed(1)}%`;
}

// trimNum mirrors the backend's old labelFor formatting (1 decimal, no
// trailing ".0") — used when building a constraint's display label
// client-side (see ResultsView's constraintLabel), now that the backend
// only sends the raw key/min/max/value and lets the UI localize the text.
export function trimNum(v: number): string {
  const s = v.toFixed(1);
  return s.endsWith('.0') ? s.slice(0, -2) : s;
}
