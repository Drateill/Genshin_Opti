import { useEffect } from 'react';
import type { ArtifactQuality } from '../api/types';
import { chardb } from '../lib/refdata';
import { useLanguage } from '../i18n';
import type { Dict } from '../i18n/translations';

function statLabel(t: Dict, key: string): string {
  return t.stats[key as keyof Dict['stats']] ?? key;
}
function statUnit(key: string): string {
  return chardb.pctStats.has(key) ? '%' : '';
}

export default function ArtifactDetailModal({ artifact, onClose }: { artifact: ArtifactQuality; onClose: () => void }) {
  const { t } = useLanguage();
  const a = artifact;
  const am = t.insights.artifactModal;

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onClose]);

  return (
    <div className="score-modal-overlay" onClick={onClose}>
      <div className="card score-modal" style={{ borderTop: '2px solid oklch(var(--acc))' }} onClick={(e) => e.stopPropagation()}>
        <button className="score-modal-close" aria-label={t.insights.scoreModal.close} onClick={onClose}>
          ✕
        </button>
        <div className="score-modal-head">
          <div className="artifact-modal-badge">{a.setShort}</div>
          <div className="score-modal-headinfo">
            <div className="score-modal-name">{a.setName}</div>
            <div className="score-modal-sub">
              {(t.slots as Record<string, string>)[a.slotKey] ?? a.slotKey} · {'★'.repeat(a.rarity)} · Lv{a.level}
            </div>
          </div>
        </div>

        <div className="score-modal-total">
          <span className="score-modal-total-label">{am.critValue}</span>
          <span className="score-modal-total-value">{a.critValue.toFixed(1)}</span>
          <span className="score-modal-total-unit">{a.rollQuality !== undefined ? am.rollQuality(a.rollQuality.toFixed(0)) : am.notRated}</span>
        </div>

        <div className="artifact-modal-stats">
          <div className="artifact-modal-stat-row main">
            <span>
              {statLabel(t, a.mainStatKey)} <span className="artifact-modal-main-tag">{am.mainStat}</span>
            </span>
            <span>
              {a.mainStatValue.toFixed(1)}
              {statUnit(a.mainStatKey)}
            </span>
          </div>
          {a.substats.map((s) => (
            <div className="artifact-modal-stat-row" key={s.key}>
              <span>{statLabel(t, s.key)}</span>
              <span>
                {s.value.toFixed(1)}
                {statUnit(s.key)}
              </span>
            </div>
          ))}
        </div>

        <div className="artifact-modal-foot">
          {a.lock && <span className="badge">{am.locked}</span>}
          <span>{a.locationName ? am.wornBy(a.locationName) : am.notEquipped}</span>
        </div>
      </div>
    </div>
  );
}
