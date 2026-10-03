import { useEffect, type CSSProperties } from 'react';
import type { CharacterInsight, ScoreComponent } from '../api/types';
import { ELEM_COLOR, type Element } from '../lib/refdata';
import { useLanguage } from '../i18n';
import type { Dict } from '../i18n/translations';

function elemStyle(element?: string): CSSProperties {
  const c = element && element in ELEM_COLOR ? ELEM_COLOR[element as Element] : '0.5 0.01 280';
  return { '--el': c } as CSSProperties;
}

function Row({ label, detail, comp }: { label: string; detail: string; comp: ScoreComponent }) {
  return (
    <div className="score-modal-row">
      <div className="score-modal-row-head">
        <span className="score-modal-row-label">{label}</span>
        <span className="score-modal-row-points">
          {comp.points.toFixed(1)} <span className="score-modal-row-weight">/ {comp.weight}</span>
        </span>
      </div>
      <div className="score-modal-row-track">
        <div className="score-modal-row-fill" style={{ width: `${Math.min(100, comp.fraction * 100)}%` }} />
      </div>
      <div className="score-modal-row-detail">{detail}</div>
    </div>
  );
}

export default function ScoreBreakdownModal({ character, onClose }: { character: CharacterInsight; onClose: () => void }) {
  const { t } = useLanguage();
  const c = character;
  const b = c.breakdown;

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onClose]);

  const sm = t.insights.scoreModal;

  return (
    <div className="score-modal-overlay" onClick={onClose}>
      <div className="card score-modal" style={elemStyle(c.element)} onClick={(e) => e.stopPropagation()}>
        <button className="score-modal-close" aria-label={sm.close} onClick={onClose}>
          ✕
        </button>
        <div className="score-modal-head">
          <div className="score-modal-avatar">
            {c.name
              .split(' ')
              .map((w) => w[0])
              .join('')
              .slice(0, 2)
              .toUpperCase()}
          </div>
          <div className="score-modal-headinfo">
            <div className="score-modal-name">{c.name}</div>
            <div className="score-modal-sub">{t.insights.roster.lvC(c.level, c.constellation)}</div>
          </div>
        </div>

        <div className="score-modal-total">
          <span className="score-modal-total-label">{sm.total}</span>
          <span className="score-modal-total-value">{c.investment.toFixed(1)}</span>
          <span className="score-modal-total-unit">/ 100</span>
        </div>

        <div className="score-modal-rows">
          <Row label={sm.level} detail={sm.levelDetail(c.level)} comp={b.level} />
          <Row
            label={sm.constellation}
            detail={sm.constellationDetail(c.constellation, c.rarity === 5 ? 2 : 6)}
            comp={b.constellation}
          />
          <Row label={sm.talents} detail={sm.talentsDetail(c.avgTalent)} comp={b.talents} />
          <Row
            label={sm.weapon}
            detail={
              c.weaponName
                ? sm.weaponDetail(c.weaponName, c.weaponLevel ?? 0, c.weaponRefinement ?? 0, c.weaponRarity === 5 ? 1 : 5)
                : sm.noWeapon
            }
            comp={b.weapon}
          />
          <Row label={sm.artifactCount} detail={sm.artifactCountDetail(c.artifactsEquipped)} comp={b.artifactCount} />
          <Row
            label={sm.artifactQuality}
            detail={sm.artifactQualityDetail(Math.round(b.artifactQuality.fraction * 100), c.avgArtifactLevel)}
            comp={b.artifactQuality}
          />
        </div>
      </div>
    </div>
  );
}
