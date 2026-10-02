import { useState, type CSSProperties, type SyntheticEvent } from 'react';
import type { RosterEntry, SetInfo, StatRange, WeaponOption } from '../api/types';
import {
  ELEMENTS, ELEM_COLOR, activeResonances, circletOpts, constraintStats, sandsOpts, gobletOpts, objectiveOpts, TOPN_OPTS,
} from '../lib/refdata';
import { useT } from '../i18n';
import type { Dict } from '../i18n/translations';

interface Props {
  roster: RosterEntry[];
  sets: SetInfo[];
  charKey: string;
  targetSetKey: string;
  targetSetKey2: string;
  dualSetMode: boolean;
  sands: string[];
  goblet: string[];
  circlet: string[];
  constraints: Record<string, StatRange>;
  topN: number;
  weaponOptions: WeaponOption[];
  weaponId: number | undefined;
  team: (string | null)[]; // 3 teammate element slots, null = unpicked
  onSetTeamSlot: (index: number, element: string | null) => void;
  onSelectWeapon: (id: number) => void;
  onSelectChar: (key: string) => void;
  onSelectSet: (key: string) => void;
  onToggleDualSetMode: () => void;
  onToggleSands: (key: string) => void;
  onToggleGoblet: (key: string) => void;
  onToggleCirclet: (key: string) => void;
  onSetConstraint: (key: string, field: 'min' | 'max', value: number | undefined) => void;
  objective: string;
  onSelectObjective: (key: string) => void;
  onSetTopN: (n: number) => void;
  includeEquippedByOthers: boolean;
  onToggleIncludeEquippedByOthers: (v: boolean) => void;
  onSolve: () => void;
  solving: boolean;
}

const ELEM_CODE: Record<string, string> = {
  Pyro: 'PY', Hydro: 'HY', Anemo: 'AN', Electro: 'EL', Dendro: 'DE', Cryo: 'CR', Geo: 'GE',
};
function elemCode(element: string): string {
  return ELEM_CODE[element] ?? element.slice(0, 2).toUpperCase();
}

// --el is read by the .elem-tile/.team-slot-row.self/.resonance-row CSS to
// tint that element (and anything nesting inside it, via inheritance) with
// its own hue instead of the app's single accent color.
function elemStyle(element: string): CSSProperties {
  return { '--el': ELEM_COLOR[element as keyof typeof ELEM_COLOR] } as CSSProperties;
}

function initialsFor(name: string): string {
  return name
    .split(' ')
    .filter(Boolean)
    .map((w) => w[0])
    .join('')
    .slice(0, 2)
    .toUpperCase();
}

// hideOnError hides a broken icon <img> (offline, blocked host) so the
// initials/short-code text already sitting underneath it shows through,
// without needing any per-row state.
function hideOnError(e: SyntheticEvent<HTMLImageElement>) {
  e.currentTarget.style.display = 'none';
}

function RowIcon({ src }: { src?: string }) {
  if (!src) return null;
  return <img src={src} alt="" className="row-icon-img" onError={hideOnError} loading="lazy" />;
}

function profileFor(t: Dict, c: RosterEntry): string {
  const base = c.known
    ? t.configure.profileKnown(c.element, c.level, c.constellation)
    : t.configure.profileUnknown(c.level, c.constellation);
  return c.known ? base : base + t.configure.notSupportedSuffix;
}

export default function ConfigureView(props: Props) {
  const t = useT();
  const [charSearch, setCharSearch] = useState('');
  const [setSearch, setSetSearch] = useState('');
  const [weaponSearch, setWeaponSearch] = useState('');
  const selected = props.roster.find((r) => r.key === props.charKey);
  const dmgKey = selected?.dmgKey || 'pyro_dmg_';
  const resonances = activeResonances(selected?.element ?? '', props.team);
  const canSolve =
    !!selected?.known &&
    !!props.targetSetKey &&
    (!props.dualSetMode || !!props.targetSetKey2) &&
    !props.solving;
  const visibleRoster = props.roster.filter((c) =>
    c.name.toLowerCase().includes(charSearch.trim().toLowerCase())
  );
  const visibleSets = props.sets.filter((s) =>
    s.name.toLowerCase().includes(setSearch.trim().toLowerCase())
  );
  const visibleWeapons = props.weaponOptions.filter((wpn) =>
    wpn.name.toLowerCase().includes(weaponSearch.trim().toLowerCase())
  );

  const solveBlockReason = props.solving
    ? null
    : !selected
    ? t.configure.selectCharacterToSolve
    : !selected.known
    ? t.configure.notSupportedReason(selected.name)
    : !props.targetSetKey
    ? t.configure.pickTargetSet
    : props.dualSetMode && !props.targetSetKey2
    ? t.configure.pickSecondTargetSet
    : null;

  return (
    <div className="fade">
      <div className="configure-header">
        <div>
          <div className="page-title">{t.configure.title}</div>
          <div className="page-sub" style={{ marginTop: 3, fontSize: 13 }}>
            {t.configure.subtitle}
          </div>
        </div>
        <div style={{ textAlign: 'right' }}>
          <button className="btn btn-primary" disabled={!canSolve} onClick={props.onSolve} style={{
            padding: '13px 26px', fontSize: '14.5px', borderRadius: 10,
            boxShadow: '0 0 26px -5px oklch(var(--acc) / .6)',
          }}>
            {t.configure.solve}
          </button>
          {solveBlockReason && <div className="solve-block-hint">{solveBlockReason}</div>}
        </div>
      </div>

      <div className="configure-grid-top">
        <div style={{ display: 'flex', flexDirection: 'column', gap: 18 }}>
        {/* character */}
        <div className="card">
          <div className="card-label">{t.configure.character}</div>
          {props.roster.length > 0 && (
            <input
              className="char-search"
              type="text"
              placeholder={t.configure.searchCharacters}
              value={charSearch}
              onChange={(e) => setCharSearch(e.target.value)}
            />
          )}
          <div className="char-list">
            {props.roster.length === 0 && (
              <div className="summary-empty" style={{ padding: '8px 0' }}>
                {t.configure.noCharactersInImport}
              </div>
            )}
            {props.roster.length > 0 && visibleRoster.length === 0 && (
              <div className="summary-empty" style={{ padding: '8px 0' }}>
                {t.configure.noCharactersMatch(charSearch)}
              </div>
            )}
            {visibleRoster.map((c) => (
              <button
                key={c.key}
                className={'row-card' + (c.key === props.charKey ? ' active' : '')}
                onClick={() => props.onSelectChar(c.key)}
                title={c.known ? undefined : t.configure.charUnsupportedTitle}
              >
                <div className={'char-avatar' + (c.rarity ? ` rarity-${c.rarity}` : '')}>
                  {initialsFor(c.name)}
                  <RowIcon src={c.icon} />
                </div>
                <div className="char-info">
                  <div className="char-name">{c.name}</div>
                  <div className="char-profile">{profileFor(t, c)}</div>
                </div>
                {c.key === props.charKey && <div className="check-mark">✓</div>}
              </button>
            ))}
          </div>
        </div>

        {/* team / elemental resonance */}
        <div className="card">
          <div className="card-label">
            {t.configure.team} <span style={{ color: 'var(--muted)' }}>{t.configure.teamTag}</span>
          </div>
          <div className="set-target-hint" style={{ marginBottom: 12 }}>
            {t.configure.teamHint}
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            <div className="team-slot-row self" style={selected?.element ? elemStyle(selected.element) : undefined}>
              <span className="team-slot-label">{t.configure.teamSlotLabel(4)}</span>
              <span className="elem-tile self">{selected?.element ? elemCode(selected.element) : '—'}</span>
              <span className="team-self-name">{selected?.name ?? '—'}</span>
              <span className="team-self-tag">
                {selected?.element ? t.configure.teamLocked(selected.element) : ''}
              </span>
            </div>
            {props.team.map((cur, i) => (
              <div className="team-slot-row" key={i}>
                <span className="team-slot-label">{t.configure.teamSlotLabel(i + 1)}</span>
                <div className="team-slot-opts">
                  {ELEMENTS.map((el) => (
                    <button
                      key={el}
                      type="button"
                      title={el}
                      aria-pressed={cur === el}
                      className={'elem-tile' + (cur === el ? ' active' : '')}
                      style={elemStyle(el)}
                      onClick={() => props.onSetTeamSlot(i, cur === el ? null : el)}
                    >
                      {elemCode(el)}
                    </button>
                  ))}
                </div>
              </div>
            ))}
          </div>
          {resonances.length > 0 && (
            <div className="resonance-box">
              <div className="card-label" style={{ marginBottom: 10 }}>
                {t.configure.activeResonance}
              </div>
              <div className="resonance-list">
                {resonances.map((r) => {
                  const info = t.resonances[r.element];
                  return (
                    <div
                      className={'resonance-row' + (r.primary ? ' primary' : '')}
                      style={elemStyle(r.element)}
                      key={r.element}
                    >
                      <span className="elem-tile self" style={{ width: 26, height: 26 }}>
                        {elemCode(r.element)}
                      </span>
                      <div style={{ flex: 1, minWidth: 0 }}>
                        <div className="resonance-title">{info.title}</div>
                        <div className="resonance-effect">{info.effect}</div>
                      </div>
                      <div className="resonance-tag">
                        {r.primary ? '' : t.configure.resonanceNotModeled}
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          )}
        </div>
        </div>

        {/* target set */}
        <div className="card">
          <div className="card-label">
            {t.configure.targetSet}{' '}
            <span className="set-target-hint-tag">
              {props.dualSetMode ? t.configure.twoPieceTag : t.configure.fourPieceTag}
            </span>
          </div>
          <div className="set-target-hint">
            {props.dualSetMode ? t.configure.targetSetHintDual : t.configure.targetSetHint}
          </div>
          <div className="chip-row" style={{ marginBottom: 10 }}>
            <button className={'chip' + (!props.dualSetMode ? ' active' : '')} onClick={props.onToggleDualSetMode}>
              {t.configure.setModeSingle}
            </button>
            <button className={'chip' + (props.dualSetMode ? ' active' : '')} onClick={props.onToggleDualSetMode}>
              {t.configure.setModeDual}
            </button>
          </div>
          {props.sets.length > 0 && (
            <input
              className="char-search"
              type="text"
              placeholder={t.configure.searchSets}
              value={setSearch}
              onChange={(e) => setSetSearch(e.target.value)}
            />
          )}
          <div className="set-list">
            {props.sets.length === 0 && (
              <div className="summary-empty" style={{ padding: '8px 0' }}>
                {t.configure.noSetsDetected}
              </div>
            )}
            {props.sets.length > 0 && visibleSets.length === 0 && (
              <div className="summary-empty" style={{ padding: '8px 0' }}>
                {t.configure.noSetsMatch(setSearch)}
              </div>
            )}
            {visibleSets.map((s) => {
              const isPrimary = s.key === props.targetSetKey;
              const isSecondary = props.dualSetMode && s.key === props.targetSetKey2;
              return (
                <button
                  key={s.key}
                  className={'set-row' + (isPrimary || isSecondary ? ' active' : '')}
                  onClick={() => props.onSelectSet(s.key)}
                  title={s.description || undefined}
                >
                  <span className="set-code">
                    {s.short}
                    <RowIcon src={s.icon} />
                  </span>
                  <span className="set-name">{s.name}</span>
                  {isPrimary && <span className="set-check">{props.dualSetMode ? '①' : '✓'}</span>}
                  {isSecondary && <span className="set-check">②</span>}
                </button>
              );
            })}
          </div>
        </div>

        {/* weapon */}
        <div className="card">
          <div className="card-label">
            {t.configure.weapon}
            {selected?.weaponType && (
              <span className="set-target-hint-tag">
                {' '}
                · {t.weaponTypes[selected.weaponType as keyof Dict['weaponTypes']] ?? selected.weaponType}
              </span>
            )}
          </div>
          <div className="set-target-hint">{t.configure.weaponHint}</div>
          {selected?.equippedWeapon && !selected.equippedWeaponKnown && (
            <div className="solve-block-hint" style={{ maxWidth: 'none', marginBottom: 10 }}>
              {t.configure.equippedNotInDb(selected.equippedWeapon)}
            </div>
          )}
          {!selected?.known && (
            <div className="summary-empty" style={{ padding: '8px 0' }}>
              {t.configure.selectSupportedForWeapons}
            </div>
          )}
          {selected?.known && props.weaponOptions.length === 0 && (
            <div className="summary-empty" style={{ padding: '8px 0' }}>
              {t.configure.noWeaponsOfType(t.weaponTypes[selected.weaponType as keyof Dict['weaponTypes']] ?? selected.weaponType)}
            </div>
          )}
          {props.weaponOptions.length > 0 && (
            <input
              className="char-search"
              type="text"
              placeholder={t.configure.searchWeapons}
              value={weaponSearch}
              onChange={(e) => setWeaponSearch(e.target.value)}
            />
          )}
          {props.weaponOptions.length > 0 && visibleWeapons.length === 0 && (
            <div className="summary-empty" style={{ padding: '8px 0' }}>
              {t.configure.noWeaponsMatch(weaponSearch)}
            </div>
          )}
          <div className="set-list">
            {visibleWeapons.map((wpn) => (
              <button
                key={wpn.id}
                className={'set-row' + (wpn.id === props.weaponId ? ' active' : '')}
                onClick={() => props.onSelectWeapon(wpn.id)}
              >
                <span className={'set-code' + (wpn.rarity ? ` rarity-${wpn.rarity}` : '')}>
                  {wpn.rarity}★
                  <RowIcon src={wpn.icon} />
                </span>
                <span className="set-name">
                  {wpn.name}
                  {wpn.count > 1 && ` ×${wpn.count}`}
                  <div className="char-profile">
                    {t.configure.weaponRow(wpn.refinement, wpn.level, wpn.atk, t.stats.atk)}
                    {wpn.subStatKey &&
                      ` · +${wpn.subStatValue} ${t.stats[wpn.subStatKey as keyof Dict['stats']] ?? wpn.subStatKey}`}
                  </div>
                </span>
                {wpn.id === props.weaponId && <span className="set-check">✓</span>}
              </button>
            ))}
          </div>
        </div>
      </div>

      <div className="configure-grid">
        {/* slot main-stats */}
        <div className="card">
          <div className="card-label">{t.configure.slotMainStats}</div>
          <div className="slot-group">
            <div className="slot-group-label">{t.configure.sands}</div>
            <div className="chip-row">
              {sandsOpts(t).map(([key, label]) => (
                <button
                  key={key}
                  className={'chip' + (props.sands.includes(key) ? ' active' : '')}
                  onClick={() => props.onToggleSands(key)}
                >
                  {label}
                </button>
              ))}
            </div>
          </div>
          <div className="slot-group">
            <div className="slot-group-label">{t.configure.goblet}</div>
            <div className="chip-row">
              {gobletOpts(t, dmgKey).map(([key, label]) => (
                <button
                  key={key}
                  className={'chip' + (props.goblet.includes(key) ? ' active' : '')}
                  onClick={() => props.onToggleGoblet(key)}
                >
                  {label}
                </button>
              ))}
            </div>
          </div>
          <div className="slot-group">
            <div className="slot-group-label">{t.configure.circlet}</div>
            <div className="chip-row">
              {circletOpts(t).map(([key, label]) => (
                <button
                  key={key}
                  className={'chip' + (props.circlet.includes(key) ? ' active' : '')}
                  onClick={() => props.onToggleCirclet(key)}
                >
                  {label}
                </button>
              ))}
            </div>
          </div>
        </div>

        {/* constraints + objective */}
        <div className="card">
          <div className="card-label">{t.configure.constraints}</div>
          <div className="constraint-input-list" style={{ marginBottom: 18 }}>
            {[...constraintStats(t), ...(selected?.dmgKey
              ? [{ key: selected.dmgKey, label: t.stats[selected.dmgKey as keyof Dict['stats']] ?? selected.dmgKey, pct: true }]
              : [])].map((s) => {
              const range = props.constraints[s.key] ?? {};
              return (
                <div className="constraint-input-row" key={s.key}>
                  <div className="constraint-input-label">{s.label}</div>
                  <div className="constraint-input-fields">
                    <input
                      type="number"
                      className="num-input"
                      placeholder={t.configure.min}
                      value={range.min ?? ''}
                      onChange={(e) =>
                        props.onSetConstraint(
                          s.key,
                          'min',
                          e.target.value === '' ? undefined : Number(e.target.value)
                        )
                      }
                    />
                    <span className="constraint-input-sep">–</span>
                    <input
                      type="number"
                      className="num-input"
                      placeholder={t.configure.max}
                      value={range.max ?? ''}
                      onChange={(e) =>
                        props.onSetConstraint(
                          s.key,
                          'max',
                          e.target.value === '' ? undefined : Number(e.target.value)
                        )
                      }
                    />
                    {s.pct && <span className="constraint-input-unit">%</span>}
                  </div>
                </div>
              );
            })}
          </div>

          <div className="card-label">{t.configure.inventory}</div>
          <label className="toggle-row" style={{ marginBottom: 18 }}>
            <div className="toggle-row-text">
              <div className="toggle-row-title">{t.configure.includeEquippedByOthers}</div>
              <div className="toggle-row-sub">
                {t.configure.includeEquippedByOthersSub(selected?.known ? selected.name : t.common.thisCharacter)}
              </div>
            </div>
            <span className="toggle-switch">
              <input
                type="checkbox"
                checked={props.includeEquippedByOthers}
                onChange={(e) => props.onToggleIncludeEquippedByOthers(e.target.checked)}
              />
              <span className="toggle-switch-track" />
            </span>
          </label>

          <div className="card-label">{t.configure.objective}</div>
          <div className="objective-row">
            {objectiveOpts(t, selected?.dmgKey ? (t.stats[selected.dmgKey as keyof Dict['stats']] ?? selected.dmgKey) : t.results.elementalDmgLabel).map((o) => (
              <button
                key={o.key}
                className={'objective-card' + (props.objective === o.key ? ' active' : '')}
                onClick={() => props.onSelectObjective(o.key)}
              >
                <div className="objective-title">{o.title}</div>
                <div className="objective-formula">{o.formula}</div>
              </button>
            ))}
            <div className="objective-card disabled">
              <div className="objective-title">{t.configure.realDmg}</div>
              <div className="objective-formula">{t.configure.realDmgSoon}</div>
            </div>
          </div>

          <div className="card-label">{t.configure.resultsToReturn}</div>
          <div className="chip-row">
            {TOPN_OPTS.map((n) => (
              <button
                key={n}
                className={'chip' + (props.topN === n ? ' active' : '')}
                onClick={() => props.onSetTopN(n)}
              >
                {t.configure.topN(n)}
              </button>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
