import { useEffect, useState } from 'react';
import { api } from '../api/client';
import type {
  BuildTotals,
  CharacterTalents,
  DamageResponse,
  EnemyPreset,
  ExtraBonus,
  ReactionType,
  TalentGroup,
  TalentLevels,
} from '../api/types';
import { localeTag, useLanguage } from '../i18n';
import type { Dict } from '../i18n/translations';

interface Props {
  characterKey: string;
  casterLevel: number;
  talentLevels: TalentLevels;
  totals: BuildTotals;
}

const TALENT_GROUPS: TalentGroup[] = ['auto', 'skill', 'burst'];
const REACTIONS: ReactionType[] = [
  '',
  'vaporize',
  'melt',
  'overloaded',
  'superconduct',
  'electrocharged',
  'swirl',
  'shattered',
  'burning',
  'bloom',
  'burgeon',
  'hyperbloom',
  'spread',
  'aggravate',
];

const EMPTY_EXTRA: ExtraBonus = {
  dmgPct: 0,
  critRatePct: 0,
  critDmgPct: 0,
  flatATK: 0,
  atkPct: 0,
  defShredPct: 0,
  resShredPct: 0,
};

function reactionLabel(t: Dict, r: ReactionType): string {
  if (r === '') return t.damage.reactionNone;
  return t.damage.reactionNames[r];
}

export default function DamageCalcPanel(props: Props) {
  const { t, lang } = useLanguage();
  const locale = localeTag(lang);

  const [talents, setTalents] = useState<CharacterTalents | null>(null);
  const [enemies, setEnemies] = useState<EnemyPreset[]>([]);
  const [group, setGroup] = useState<TalentGroup>('skill');
  const [talentLevel, setTalentLevel] = useState(props.talentLevels.skill || 1);
  const [casterLevel, setCasterLevel] = useState(props.casterLevel);
  const [selectedIdx, setSelectedIdx] = useState<Set<number> | null>(null); // null = every component
  const [enemyPresetKey, setEnemyPresetKey] = useState('');
  const [enemyLevel, setEnemyLevel] = useState(90);
  const [enemyRes, setEnemyRes] = useState(10);
  const [reaction, setReaction] = useState<ReactionType>('');
  const [reactionBonus, setReactionBonus] = useState(0);
  const [extra, setExtra] = useState<ExtraBonus>(EMPTY_EXTRA);
  const [result, setResult] = useState<DamageResponse | null>(null);

  // Re-fetch this character's talent multipliers whenever the character or
  // display language changes, and reset the talent group/level/selection
  // back to sensible defaults for the new character.
  useEffect(() => {
    let cancelled = false;
    api
      .characterTalents(props.characterKey, lang)
      .then((data) => {
        if (cancelled) return;
        setTalents(data);
      })
      .catch(() => {
        if (!cancelled) setTalents(null);
      });
    setGroup('skill');
    setSelectedIdx(null);
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [props.characterKey, lang]);

  useEffect(() => {
    setTalentLevel(props.talentLevels[group] || 1);
    setSelectedIdx(null);
  }, [group, props.talentLevels]);

  useEffect(() => {
    setCasterLevel(props.casterLevel);
  }, [props.casterLevel, props.characterKey]);

  useEffect(() => {
    api
      .enemies(lang)
      .then(setEnemies)
      .catch(() => setEnemies([]));
  }, [lang]);

  const components = talents?.[group] ?? [];

  useEffect(() => {
    if (components.length === 0) {
      setResult(null);
      return;
    }
    const componentIndices = selectedIdx ? Array.from(selectedIdx) : undefined;
    const timer = setTimeout(() => {
      api
        .calcDamage({
          characterKey: props.characterKey,
          casterLevel,
          totals: props.totals,
          talentGroup: group,
          talentLevel,
          componentIndices,
          extra,
          enemy: { level: enemyLevel, resPct: enemyRes },
          reaction: { type: reaction, bonusPct: reactionBonus },
        })
        .then(setResult)
        .catch(() => setResult(null));
    }, 150);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    props.characterKey,
    casterLevel,
    props.totals,
    group,
    talentLevel,
    selectedIdx,
    extra,
    enemyLevel,
    enemyRes,
    reaction,
    reactionBonus,
    components.length,
  ]);

  function toggleComponent(i: number) {
    setSelectedIdx((cur) => {
      const all = components.length;
      const next = new Set(cur ?? Array.from({ length: all }, (_, k) => k));
      if (next.has(i)) next.delete(i);
      else next.add(i);
      return next.size === all ? null : next;
    });
  }

  function selectEnemyPreset(key: string) {
    setEnemyPresetKey(key);
    const preset = enemies.find((e) => e.key === key);
    if (preset) {
      setEnemyLevel(preset.level);
      setEnemyRes(preset.resPct);
    }
  }

  function setExtraField(key: keyof ExtraBonus, value: number) {
    setExtra((cur) => ({ ...cur, [key]: value }));
  }

  return (
    <details className="accordion" style={{ marginTop: 30 }}>
      <summary>{t.damage.title}</summary>
      <div className="accordion-body">
        <div className="constraints-card" style={{ marginBottom: 14, fontSize: 12, opacity: 0.85 }}>
          {t.damage.approxNote}
        </div>

        <div className="totals-card" style={{ marginBottom: 14 }}>
          <div className="totals-card-label">{t.damage.talentGroup}</div>
          <div className="chip-row" style={{ marginBottom: 14 }}>
            {TALENT_GROUPS.map((g) => (
              <button key={g} className={'chip' + (group === g ? ' active' : '')} onClick={() => setGroup(g)}>
                {t.damage[g]}
              </button>
            ))}
          </div>

          <div className="constraint-input-row" style={{ marginBottom: 14 }}>
            <span className="constraint-input-label">{t.damage.talentLevel}</span>
            <input
              className="num-input"
              type="number"
              min={1}
              max={15}
              value={talentLevel}
              onChange={(e) => setTalentLevel(Math.max(1, Math.min(15, Number(e.target.value) || 1)))}
            />
          </div>

          <div className="totals-card-label">{t.damage.components}</div>
          {components.length === 0 ? (
            <div className="empty-state-body" style={{ padding: 0 }}>
              {t.damage.noComponents}
            </div>
          ) : (
            <div className="chip-row" style={{ marginBottom: 14 }}>
              {components.map((c, i) => (
                <button
                  key={i}
                  className={'chip' + (!selectedIdx || selectedIdx.has(i) ? ' active' : '')}
                  onClick={() => toggleComponent(i)}
                >
                  {c.label} · {c.values[talentLevel - 1]?.toFixed(1)}%
                </button>
              ))}
            </div>
          )}

          <div className="constraint-input-row" style={{ marginBottom: 4 }}>
            <span className="constraint-input-label">{t.damage.casterLevel}</span>
            <div className="constraint-input-fields">
              <input
                className="num-input"
                type="number"
                min={1}
                max={90}
                value={casterLevel}
                onChange={(e) => setCasterLevel(Math.max(1, Math.min(90, Number(e.target.value) || 1)))}
              />
              {casterLevel < 90 && (
                <button
                  className="slot-upgrade-btn"
                  onClick={() => setCasterLevel(90)}
                  title={t.damage.casterLevelToMaxTitle}
                >
                  {t.damage.casterLevelToMax}
                </button>
              )}
            </div>
          </div>
        </div>

        <div className="totals-card" style={{ marginBottom: 14 }}>
          <div className="totals-card-label">{t.damage.enemy}</div>
          <div className="constraint-input-list">
            <div className="constraint-input-row">
              <span className="constraint-input-label">{t.damage.enemyPreset}</span>
              <select
                className="num-input"
                style={{ width: 'auto', textAlign: 'left' }}
                value={enemyPresetKey}
                onChange={(e) => selectEnemyPreset(e.target.value)}
              >
                <option value="">{t.damage.customEnemy}</option>
                {enemies.map((e) => (
                  <option key={e.key} value={e.key}>
                    {e.name} (Lv{e.level})
                  </option>
                ))}
              </select>
            </div>
            <div className="constraint-input-row">
              <span className="constraint-input-label">{t.damage.enemyLevel}</span>
              <input
                className="num-input"
                type="number"
                min={1}
                max={100}
                value={enemyLevel}
                onChange={(e) => {
                  setEnemyPresetKey('');
                  setEnemyLevel(Number(e.target.value) || 0);
                }}
              />
            </div>
            <div className="constraint-input-row">
              <span className="constraint-input-label">{t.damage.enemyRes}</span>
              <input
                className="num-input"
                type="number"
                value={enemyRes}
                onChange={(e) => {
                  setEnemyPresetKey('');
                  setEnemyRes(Number(e.target.value) || 0);
                }}
              />
            </div>
            <div className="constraint-input-row">
              <span className="constraint-input-label">{t.damage.defShred}</span>
              <input
                className="num-input"
                type="number"
                value={extra.defShredPct}
                onChange={(e) => setExtraField('defShredPct', Number(e.target.value) || 0)}
              />
            </div>
            <div className="constraint-input-row">
              <span className="constraint-input-label">{t.damage.resShred}</span>
              <input
                className="num-input"
                type="number"
                value={extra.resShredPct}
                onChange={(e) => setExtraField('resShredPct', Number(e.target.value) || 0)}
              />
            </div>
          </div>
        </div>

        <div className="totals-card" style={{ marginBottom: 14 }}>
          <div className="totals-card-label">{t.damage.reaction}</div>
          <div className="constraint-input-list">
            <div className="constraint-input-row">
              <span className="constraint-input-label">{t.damage.reaction}</span>
              <select
                className="num-input"
                style={{ width: 'auto', textAlign: 'left' }}
                value={reaction}
                onChange={(e) => setReaction(e.target.value as ReactionType)}
              >
                {REACTIONS.map((r) => (
                  <option key={r} value={r}>
                    {reactionLabel(t, r)}
                  </option>
                ))}
              </select>
            </div>
            {reaction !== '' && (
              <div className="constraint-input-row">
                <span className="constraint-input-label">{t.damage.reactionBonus}</span>
                <input
                  className="num-input"
                  type="number"
                  value={reactionBonus}
                  onChange={(e) => setReactionBonus(Number(e.target.value) || 0)}
                />
              </div>
            )}
          </div>
        </div>

        <div className="totals-card" style={{ marginBottom: 14 }}>
          <div className="totals-card-label">{t.damage.extraBonus}</div>
          <div className="totals-values">
            {(
              [
                ['dmgPct', t.damage.dmgPct],
                ['critRatePct', t.damage.critRatePct],
                ['critDmgPct', t.damage.critDmgPct],
                ['flatATK', t.damage.flatAtk],
                ['atkPct', t.damage.atkPct],
              ] as [keyof ExtraBonus, string][]
            ).map(([key, label]) => (
              <div key={key}>
                <div className="total-item-label">{label}</div>
                <input
                  className="num-input"
                  type="number"
                  style={{ width: '100%' }}
                  value={extra[key]}
                  onChange={(e) => setExtraField(key, Number(e.target.value) || 0)}
                />
              </div>
            ))}
          </div>
        </div>

        {result && (
          <div className="constraints-card">
            <div className="constraints-card-label">{t.damage.title}</div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 11 }}>
              {result.components.map((c, i) => (
                <div className="constraint-row" key={i}>
                  <span className="constraint-label">{c.label}</span>
                  <span className="constraint-met">
                    {t.damage.average}: {Math.round(c.average).toLocaleString(locale)} · {t.damage.nonCrit}:{' '}
                    {Math.round(c.nonCrit).toLocaleString(locale)} · {t.damage.critLabel}:{' '}
                    {Math.round(c.crit).toLocaleString(locale)}
                  </span>
                </div>
              ))}
              {result.components.length > 1 && (
                <div className="constraint-row">
                  <span className="constraint-label">{t.damage.total}</span>
                  <span className="constraint-met">{Math.round(result.total).toLocaleString(locale)}</span>
                </div>
              )}
              {!!result.reactionDamage && (
                <div className="constraint-row">
                  <span className="constraint-label">{t.damage.reactionDamage}</span>
                  <span className="constraint-met">{Math.round(result.reactionDamage).toLocaleString(locale)}</span>
                </div>
              )}
            </div>
          </div>
        )}
      </div>
    </details>
  );
}
