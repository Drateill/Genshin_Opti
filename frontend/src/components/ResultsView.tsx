import { useEffect, useState } from 'react';
import type {
  Artifact,
  BuildResult,
  BuildTotals,
  ConstraintCheck,
  RosterEntry,
  SetInfo,
  SolveResponse,
  WeaponOption,
} from '../api/types';
import { SLOT_TAG, chardb } from '../lib/refdata';
import { fmtStat, trimNum } from '../lib/format';
import { applyMainStatUpgrade, applyWeaponUpgrade, mainStatAtLevel20 } from '../lib/buildmath';
import { localeTag, useLanguage } from '../i18n';
import type { Dict } from '../i18n/translations';
import DamageCalcPanel from './DamageCalcPanel';

interface Props {
  results: SolveResponse | null;
  solveError: string | null;
  targetSetKey: string;
  targetSetKey2?: string;
  sets: SetInfo[];
  roster: RosterEntry[];
  charKey: string;
  weapon?: WeaponOption;
  selIdx: number;
  onSelectRank: (i: number) => void;
  onBackToConfigure: () => void;
  showSolverStats: boolean;
}

const SLOT_ORDER = ['flower', 'plume', 'sands', 'goblet', 'circlet'] as const;

function setShort(sets: SetInfo[], key: string): string {
  return sets.find((s) => s.key === key)?.short ?? key.slice(0, 2).toUpperCase();
}

function setName(sets: SetInfo[], key: string): string {
  return sets.find((s) => s.key === key)?.name ?? key;
}

function pieceForSlot(pieces: Artifact[], slot: string): Artifact | undefined {
  return pieces.find((p) => p.slotKey === slot);
}

function statLabel(t: Dict, key: string): string {
  return t.stats[key as keyof Dict['stats']] ?? key;
}

// constraintLabel mirrors the backend's old labelFor formatting, now built
// client-side (in the viewer's language) from the plain key/min/max/value
// the API sends — see model.ConstraintCheck's doc comment.
function constraintLabel(t: Dict, c: ConstraintCheck): string {
  const name = statLabel(t, c.key);
  const unit = chardb.pctStats.has(c.key) ? '%' : '';
  if (c.min !== undefined && c.max !== undefined) return `${name} ${trimNum(c.min)}${unit}–${trimNum(c.max)}${unit}`;
  if (c.min !== undefined) return `${name} ≥ ${trimNum(c.min)}${unit}`;
  if (c.max !== undefined) return `${name} ≤ ${trimNum(c.max)}${unit}`;
  return name;
}

// Who (if anyone) currently has this piece equipped, for the "worn by
// someone else" heads-up on a proposed build's slot cards. Not shown on the
// current-build grid, since every piece there is by definition on charKey.
function equipBadge(
  t: Dict,
  a: Artifact,
  charKey: string,
  roster: RosterEntry[]
): { text: string; own: boolean } | null {
  if (!a.location) return null;
  if (a.location === charKey) return { text: t.results.alreadyEquipped, own: true };
  const owner = roster.find((r) => r.key === a.location);
  return { text: t.results.wornBy(owner ? owner.name : a.location), own: false };
}

interface SlotCardOpts {
  t: Dict;
  locale: string;
  targetSetKey: string;
  sets: SetInfo[];
  showEquip?: { charKey: string; roster: RosterEntry[] };
  onUpgrade?: (slot: string) => void;
}

function renderSlotCard(slot: string, a: Artifact | undefined, opts: SlotCardOpts) {
  const { t } = opts;
  if (!a) {
    return (
      <div className="slot-card slot-card-empty" key={slot}>
        <div className="slot-card-head">
          <div className="slot-tag">{SLOT_TAG[slot]}</div>
          <span className="slot-name">{t.slots[slot as keyof Dict['slots']] ?? slot}</span>
        </div>
        <div className="slot-empty-label">{t.results.notEquipped}</div>
      </div>
    );
  }
  const offSet = a.setKey !== opts.targetSetKey;
  const badge = opts.showEquip ? equipBadge(t, a, opts.showEquip.charKey, opts.showEquip.roster) : null;
  const canUpgrade =
    !!opts.onUpgrade && a.rarity === 5 && a.level < 20 && mainStatAtLevel20(a.mainStatKey, a.rarity) !== undefined;
  return (
    <div className="slot-card" key={slot}>
      <div className="slot-card-head">
        <div className="slot-tag">{SLOT_TAG[slot]}</div>
        <span className="slot-name">{t.slots[slot as keyof Dict['slots']] ?? slot}</span>
        <span className="slot-level">+{a.level}</span>
        {offSet && <span className="slot-offset-badge">{setShort(opts.sets, a.setKey)}</span>}
      </div>
      <div className="slot-main-row">
        <div>
          <div className="slot-main-label">{statLabel(t, a.mainStatKey)}</div>
          <div className="slot-main-value">{fmtStat(a.mainStatKey, a.mainStatValue, opts.locale)}</div>
        </div>
        {canUpgrade && (
          <button
            className="slot-upgrade-btn"
            onClick={() => opts.onUpgrade!(slot)}
            title={t.results.upgradeTo20Title}
          >
            {t.results.upgradeTo20}
          </button>
        )}
      </div>
      <div className="slot-subs">
        {a.substats.map((sub, j) => (
          <div className="slot-sub" key={j}>
            <span className="slot-sub-label">
              {(sub.key === 'critRate_' || sub.key === 'critDMG_') && <span className="slot-sub-dot" />}
              {statLabel(t, sub.key)}
            </span>
            <span className="slot-sub-value">+{fmtStat(sub.key, sub.value, opts.locale)}</span>
          </div>
        ))}
      </div>
      {badge && (
        <div className={'slot-equip-badge' + (badge.own ? ' slot-equip-badge-own' : ' slot-equip-badge-other')}>
          {badge.text}
        </div>
      )}
    </div>
  );
}

interface CompareRow {
  key: string;
  label: (t: Dict) => string;
  value: (t: BuildTotals, cv: number) => number;
  fmt: (v: number, locale: string) => string;
}

const COMPARE_ROWS: CompareRow[] = [
  { key: 'cv', label: (t) => t.results.critValueLabel, value: (_t, cv) => cv, fmt: (v) => v.toFixed(1) },
  { key: 'cr', label: (t) => t.stats.critRate_, value: (t) => t.critRate, fmt: (v) => `${v.toFixed(1)}%` },
  { key: 'cd', label: (t) => t.stats.critDMG_, value: (t) => t.critDMG, fmt: (v) => `${v.toFixed(1)}%` },
  { key: 'em', label: (t) => t.stats.em, value: (t) => t.elementalMastery, fmt: (v) => Math.round(v).toString() },
  { key: 'atk', label: (t) => t.stats.atk, value: (t) => t.atk, fmt: (v, locale) => Math.round(v).toLocaleString(locale) },
  { key: 'er', label: (t) => t.stats.enerRech_, value: (t) => t.energyRecharge, fmt: (v) => `${v.toFixed(1)}%` },
  { key: 'edmg', label: (t) => t.results.elementalDmgLabel, value: (t) => t.elementalDMG, fmt: (v) => `${v.toFixed(1)}%` },
];

function fmtDelta(fmt: (v: number, locale: string) => string, delta: number, locale: string): string {
  const text = fmt(Math.abs(delta), locale);
  if (delta > 1e-6) return `+${text}`;
  if (delta < -1e-6) return `-${text}`;
  return fmt(0, locale);
}

export default function ResultsView(props: Props) {
  const { t, lang } = useLanguage();
  const locale = localeTag(lang);
  const { results } = props;
  const builds = results?.builds ?? [];
  const hasResults = builds.length > 0;

  // "to 20" is a local preview only — it never touches results, just layers
  // a recomputed BuildResult on top of whichever one is on screen. Switching
  // ranks or re-solving drops the preview rather than carrying stale numbers.
  const [selectedOverride, setSelectedOverride] = useState<BuildResult | null>(null);
  const [currentOverride, setCurrentOverride] = useState<BuildResult | null>(null);
  useEffect(() => setSelectedOverride(null), [props.selIdx, results]);
  useEffect(() => setCurrentOverride(null), [results]);

  const metaText = results
    ? t.results.metaCount(builds.length) +
      (props.showSolverStats ? t.results.metaSolveStats(results.solveMs, results.prunedBranches) : '')
    : '';

  const maxCv = builds[0]?.critValue ?? 1;
  const minCv = builds[builds.length - 1]?.critValue ?? 0;
  const span = Math.max(maxCv - minCv, 1e-6);

  const selectedFound: BuildResult | undefined = builds[Math.min(props.selIdx, builds.length - 1)];
  const selected = selectedOverride ?? selectedFound;
  const currentBuild = currentOverride ?? results?.currentBuild;
  const charEntry = props.roster.find((r) => r.key === props.charKey);
  const dmgKey = charEntry?.dmgKey ?? '';
  const weapon = props.weapon;
  const canUpgradeWeapon = !!weapon && weapon.known && weapon.level < 90 && weapon.atk > 0;

  return (
    <div className="fade">
      <div className="results-header">
        <div className="results-title-row">
          <span className="results-title">{t.results.title}</span>
          <span className="results-meta">{metaText}</span>
        </div>
        <button className="btn btn-secondary" onClick={props.onBackToConfigure}>
          {t.results.editConfiguration}
        </button>
      </div>

      {props.solveError && <div className="import-error">{props.solveError}</div>}
      {results?.timedOut && <div className="import-error">{t.results.timedOut}</div>}

      {hasResults && selected ? (
        <div>
          <div className="rank-list">
            {builds.map((b, i) => {
              const failed = b.checks.filter((c) => !c.met).length;
              const barPct = builds.length < 2 ? 100 : Math.round(22 + ((b.critValue - minCv) / span) * 78);
              return (
                <button
                  key={i}
                  className={'rank-row' + (i === props.selIdx ? ' selected' : '')}
                  onClick={() => props.onSelectRank(i)}
                >
                  <div className="rank-num">{i + 1}</div>
                  <div>
                    <div className="rank-cv-row">
                      <span className="rank-cv-value">{b.critValue.toFixed(1)}</span>
                      <span className="rank-cv-label">CV</span>
                    </div>
                    <div className="rank-cv-bar">
                      <div className="rank-cv-bar-fill" style={{ width: `${barPct}%` }} />
                    </div>
                  </div>
                  <div className="rank-stats">
                    <div>{t.results.critAbbrevRow(b.totals.critRate.toFixed(1), b.totals.critDMG.toFixed(1))}</div>
                    <div>{t.results.emAbbrevRow(Math.round(b.totals.elementalMastery).toString())}</div>
                  </div>
                  <div className="rank-status-cell">
                    {b.allMet ? (
                      <span className="rank-status-pass">{t.results.pass}</span>
                    ) : (
                      <span className="rank-status-fail">{t.results.missed(failed)}</span>
                    )}
                  </div>
                </button>
              );
            })}
          </div>

          <div className="detail-header">
            <span className="detail-rank-badge">{t.results.rank(props.selIdx + 1)}</span>
            <span className="detail-title">{t.results.buildDetail}</span>
            <span className="detail-setlabel">
              {setName(props.sets, props.targetSetKey)} ×{selected.onSetCount}
              {props.targetSetKey2 &&
                ` + ${setName(props.sets, props.targetSetKey2)} ×${selected.onSetCount2 ?? 0}`}
            </span>
            {canUpgradeWeapon && (
              <button
                className="slot-upgrade-btn"
                onClick={() => setSelectedOverride(applyWeaponUpgrade(selected, weapon!.atk, weapon!.level, dmgKey))}
                title={t.results.upgradeWeaponToMaxTitle}
              >
                {t.results.upgradeWeaponToMax}
              </button>
            )}
            {selectedOverride && (
              <button className="preview-reset-btn" onClick={() => setSelectedOverride(null)}>
                {t.results.resetToActualLevels}
              </button>
            )}
          </div>

          <div className="slot-grid">
            {SLOT_ORDER.map((slot) =>
              renderSlotCard(slot, pieceForSlot(selected.pieces, slot), {
                t,
                locale,
                targetSetKey: props.targetSetKey,
                sets: props.sets,
                showEquip: { charKey: props.charKey, roster: props.roster },
                onUpgrade: (s) => setSelectedOverride(applyMainStatUpgrade(selected, s, dmgKey)),
              })
            )}
          </div>

          <div className="totals-grid">
            <div className="totals-card">
              <div className="totals-card-label">{t.results.totalBuildStats}</div>
              <div className="totals-values">
                <div>
                  <div className="total-item-label">{t.results.critValueLabel}</div>
                  <div className="total-item-value">{selected.critValue.toFixed(1)}</div>
                </div>
                <div>
                  <div className="total-item-label">{t.stats.critRate_}</div>
                  <div className="total-item-value">{selected.totals.critRate.toFixed(1)}%</div>
                </div>
                <div>
                  <div className="total-item-label">{t.stats.critDMG_}</div>
                  <div className="total-item-value">{selected.totals.critDMG.toFixed(1)}%</div>
                </div>
                <div>
                  <div className="total-item-label">{t.stats.em}</div>
                  <div className="total-item-value">{Math.round(selected.totals.elementalMastery)}</div>
                </div>
                <div>
                  <div className="total-item-label">{t.stats.atk}</div>
                  <div className="total-item-value">{Math.round(selected.totals.atk).toLocaleString(locale)}</div>
                </div>
                <div>
                  <div className="total-item-label">{t.stats.enerRech_}</div>
                  <div className="total-item-value">{selected.totals.energyRecharge.toFixed(1)}%</div>
                </div>
                <div>
                  <div className="total-item-label">{t.results.elementalDmgLabel}</div>
                  <div className="total-item-value">{selected.totals.elementalDMG.toFixed(1)}%</div>
                </div>
              </div>
            </div>
            <div className="constraints-card">
              <div className="constraints-card-label">{t.configure.constraints}</div>
              {selected.checks.length === 0 && (
                <div className="constraint-label" style={{ opacity: 0.7 }}>
                  {t.results.noMinimumsSet}
                </div>
              )}
              {selected.checks.map((chk) => (
                <div className="constraint-row" key={chk.key}>
                  <span className="constraint-label">{constraintLabel(t, chk)}</span>
                  {chk.met ? (
                    <span className="constraint-met">✓ {fmtStat(chk.key, chk.value, locale)}</span>
                  ) : (
                    <span className="constraint-unmet">✕ {fmtStat(chk.key, chk.value, locale)}</span>
                  )}
                </div>
              ))}
            </div>
          </div>

          {charEntry?.known && (
            <DamageCalcPanel
              characterKey={props.charKey}
              casterLevel={charEntry.level}
              talentLevels={charEntry.talent}
              totals={selected.totals}
            />
          )}

          <div className="detail-header" style={{ marginTop: 30 }}>
            <span className="detail-title">{t.results.currentlyEquipped}</span>
            {currentBuild && !currentBuild.complete && (
              <span className="detail-setlabel">{t.results.slotsEquipped(currentBuild.pieces.length)}</span>
            )}
            {canUpgradeWeapon && currentBuild && (
              <button
                className="slot-upgrade-btn"
                onClick={() =>
                  setCurrentOverride(applyWeaponUpgrade(currentBuild, weapon!.atk, weapon!.level, dmgKey))
                }
                title={t.results.upgradeWeaponToMaxTitle}
              >
                {t.results.upgradeWeaponToMax}
              </button>
            )}
            {currentOverride && (
              <button className="preview-reset-btn" onClick={() => setCurrentOverride(null)}>
                {t.results.resetToActualLevels}
              </button>
            )}
          </div>

          {!currentBuild ? (
            <div className="summary-empty" style={{ padding: '8px 0' }}>
              {t.results.noArtifactsEquipped(
                props.roster.find((r) => r.key === props.charKey)?.name ?? t.common.thisCharacter
              )}
            </div>
          ) : (
            <>
              <div className="slot-grid">
                {SLOT_ORDER.map((slot) =>
                  renderSlotCard(slot, pieceForSlot(currentBuild.pieces, slot), {
                    t,
                    locale,
                    targetSetKey: props.targetSetKey,
                    sets: props.sets,
                    onUpgrade: (s) => setCurrentOverride(applyMainStatUpgrade(currentBuild, s, dmgKey)),
                  })
                )}
              </div>

              <div className="compare-card">
                <div className="compare-card-label">{t.results.compareCardLabel}</div>
                <div className="compare-table">
                  <div className="compare-row compare-row-head">
                    <span />
                    <span>{t.results.current}</span>
                    <span>{t.results.selected}</span>
                    <span>{t.results.delta}</span>
                  </div>
                  {COMPARE_ROWS.map((row) => {
                    const curV = row.value(currentBuild.totals, currentBuild.critValue);
                    const selV = row.value(selected.totals, selected.critValue);
                    const delta = selV - curV;
                    const trend = delta > 1e-6 ? 'up' : delta < -1e-6 ? 'down' : 'flat';
                    return (
                      <div className="compare-row" key={row.key}>
                        <span className="compare-row-label">{row.label(t)}</span>
                        <span className="compare-row-value">{row.fmt(curV, locale)}</span>
                        <span className="compare-row-value">{row.fmt(selV, locale)}</span>
                        <span className={'compare-row-delta compare-row-delta-' + trend}>
                          {fmtDelta(row.fmt, delta, locale)}
                        </span>
                      </div>
                    );
                  })}
                </div>
              </div>
            </>
          )}
        </div>
      ) : (
        results && (
          <div className="empty-state">
            <div className="empty-state-title">{t.results.noBuildsMatch}</div>
            <div className="empty-state-body">{results.reason}</div>
          </div>
        )
      )}
    </div>
  );
}
