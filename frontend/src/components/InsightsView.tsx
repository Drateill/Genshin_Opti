import { useEffect, useState, type CSSProperties } from 'react';
import { api, ApiError } from '../api/client';
import type { ArtifactQuality, CharacterInsight, IdleWeaponGroup, InsightsResponse } from '../api/types';
import { ELEM_COLOR, ELEMENTS, elemCode, abbrev, type Element } from '../lib/refdata';
import { useLanguage } from '../i18n';
import type { Dict } from '../i18n/translations';
import ScoreBreakdownModal from './ScoreBreakdownModal';

// Score tiers for the roster cards — S lights up in the app's own accent,
// A/B/C step down through neutral inks so only a truly built character
// pulls the eye.
const TIERS = [
  { tier: 'S', min: 80, c: 'oklch(var(--acc-lt))' },
  { tier: 'A', min: 65, c: 'oklch(0.78 0.12 190)' },
  { tier: 'B', min: 45, c: 'oklch(0.9 0.01 280)' },
  { tier: 'C', min: 0, c: 'oklch(0.72 0.01 280)' },
];
function tierColor(score: number): string {
  return (TIERS.find((t) => score >= t.min) ?? TIERS[TIERS.length - 1]).c;
}

function elemStyle(element?: string): CSSProperties {
  const c = element && element in ELEM_COLOR ? ELEM_COLOR[element as Element] : '0.5 0.01 280';
  return { '--el': c } as CSSProperties;
}

function weaponTypeLabel(t: Dict, type: string): string {
  return (t.weaponTypes as Record<string, string>)[type] ?? type;
}

function pct(n: number, total: number): number {
  return total ? Math.round((n / total) * 100) : 0;
}

function Kpi({ label, value, sub }: { label: string; value: string | number; sub?: string }) {
  return (
    <div className="ins-kpi">
      <span className="ins-kpi-label">{label}</span>
      <span className="ins-kpi-value">{value}</span>
      {sub && <span className="ins-kpi-sub">{sub}</span>}
    </div>
  );
}

interface RailItem {
  key: string;
  icon: string;
  title: string;
  sub: string;
  val: string;
  tag: string;
  alert?: boolean;
}

function RailGroup({
  t,
  title,
  hint,
  color,
  items,
  open,
  onToggle,
}: {
  t: Dict;
  title: string;
  hint: string;
  color: string;
  items: RailItem[];
  open: boolean;
  onToggle: () => void;
}) {
  const shown = open ? items : items.slice(0, 3);
  return (
    <div className="ins-rail-group">
      <div className="ins-rail-group-head">
        <span className="ins-rail-dot" style={{ background: color, boxShadow: `0 0 8px ${color}` }} />
        <span className="ins-rail-group-title">{title}</span>
        <span className="ins-rail-count" style={{ background: `color-mix(in oklab, ${color} 18%, transparent)`, color }}>
          {items.length}
        </span>
      </div>
      <div className="ins-rail-hint">{hint}</div>
      {items.length === 0 ? (
        <div className="ins-rail-empty">{t.insights.actionQueue.none}</div>
      ) : (
        <>
          <div className="ins-rail-items">
            {shown.map((it) => (
              <div className="ins-rail-item" key={it.key}>
                <span className="ins-rail-item-icon">{it.icon}</span>
                <div className="ins-rail-item-info">
                  <div className="ins-rail-item-title">{it.title}</div>
                  <div className="ins-rail-item-sub">{it.sub}</div>
                </div>
                <div className="ins-rail-item-val">
                  <span className="ins-rail-item-value" style={{ color }}>
                    {it.val}
                  </span>
                  <span
                    className="ins-rail-item-tag"
                    style={it.alert ? { color: 'var(--red)', fontWeight: 700, textTransform: 'uppercase' } : undefined}
                  >
                    {it.tag}
                  </span>
                </div>
              </div>
            ))}
          </div>
          {items.length > 3 && (
            <div role="button" className="ins-rail-more" onClick={onToggle}>
              {open ? t.insights.actionQueue.collapse : t.insights.actionQueue.more(items.length - 3)}
            </div>
          )}
        </>
      )}
    </div>
  );
}

export default function InsightsView() {
  const { t, lang } = useLanguage();
  const [data, setData] = useState<InsightsResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filterEl, setFilterEl] = useState<string>('All');
  const [showAllRoster, setShowAllRoster] = useState(false);
  const [openGroups, setOpenGroups] = useState<Record<string, boolean>>({});
  const [selectedChar, setSelectedChar] = useState<CharacterInsight | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    api
      .insights(lang)
      .then((res) => {
        if (!cancelled) setData(res);
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof ApiError ? e.message : t.insights.loadFailed);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [lang]);

  if (loading) {
    return (
      <div>
        <div className="ins-header">
          <span className="ins-header-title">{t.insights.title}</span>
          <span className="ins-header-sub">{t.insights.subtitle}</span>
        </div>
        <div className="summary-empty">{t.insights.loading}</div>
      </div>
    );
  }

  if (error || !data || data.characters.length === 0) {
    return (
      <div className="empty-state">
        <div className="empty-state-title">{t.insights.emptyTitle}</div>
        <div className="empty-state-body">{error ?? t.insights.emptyBody}</div>
      </div>
    );
  }

  const o = data.overview;
  // Stella Fortuna lets a character push past the normal level-90 cap (to
  // 95 or 100), so "max level" means >= 90, not exactly 90.
  const lv90 = data.characters.filter((c) => c.level >= 90).length;

  const kpis: { label: string; value: string | number; sub: string }[] = [
    { label: t.insights.kpi.characters, value: o.characters, sub: '' },
    { label: t.insights.kpi.artifacts, value: o.artifacts.toLocaleString(), sub: t.insights.kpi.artifactsSub(o.fiveStarArtifacts) },
    { label: t.insights.kpi.weapons, value: o.weapons, sub: t.insights.kpi.weaponsSub(o.equippedWeapons, o.benchedWeapons) },
    { label: t.insights.kpi.locked, value: o.lockedArtifacts, sub: t.insights.kpi.lockedSub(pct(o.lockedArtifacts, o.artifacts)) },
    { label: t.insights.kpi.equipped, value: o.equippedArtifacts, sub: t.insights.kpi.equippedSub(o.benchedArtifacts) },
    { label: t.insights.kpi.lv90, value: lv90, sub: t.insights.kpi.lv90Sub },
    { label: t.insights.kpi.fiveStar, value: `${pct(o.fiveStarArtifacts, o.artifacts)}%`, sub: t.insights.kpi.fiveStarSub },
    { label: t.insights.kpi.avgCV, value: data.artifactQuality.avgCritValue, sub: t.insights.kpi.avgCVSub },
  ];

  // --- elements: stacked segment bar + legend ---
  const elemTotal = data.elements.reduce((a, e) => a + e.count, 0) || 1;

  // --- weapons: dot matrix per type ---
  const maxDots = 24;
  const weaponRows = data.weaponTypes.map((w) => {
    const total = w.equipped + w.benched;
    const scale = total > maxDots ? maxDots / total : 1;
    const eqDots = Math.round(w.equipped * scale);
    const idleDots = Math.max(0, Math.round(total * scale) - eqDots);
    return { ...w, total, eqDots, idleDots, heavyIdle: w.benched > w.equipped * 2.5 && w.benched > 4 };
  });

  // --- roll quality buckets ---
  const maxBucket = Math.max(1, ...data.rollQualityBuckets.map((b) => b.count));

  // --- sets wall ---
  const topSets = data.sets.slice(0, 12);
  const setTotal = topSets.reduce((a, s) => a + s.count, 0) || 1;

  // --- roster ---
  const filtered = filterEl === 'All' ? data.characters : data.characters.filter((c) => c.element === filterEl);
  const rosterShown = showAllRoster ? filtered : filtered.slice(0, 15);
  const filterChips = [
    { key: 'All', label: t.insights.roster.all, n: data.characters.length },
    ...ELEMENTS.map((el) => ({ key: el, label: elemCode(el), n: data.characters.filter((c) => c.element === el).length })),
  ];

  // --- action queue ---
  const artQualityItem = (a: ArtifactQuality, subLabel: string): RailItem => ({
    key: String(a.id),
    icon: a.setShort,
    title: a.setName,
    sub: `${(t.slots as Record<string, string>)[a.slotKey] ?? a.slotKey} · ${abbrev(t, a.mainStatKey)} · ${subLabel}`,
    val: a.critValue.toFixed(1),
    tag: a.rollQuality !== undefined ? t.insights.actionQueue.rv(a.rollQuality.toFixed(0)) : '—',
  });
  const idleItem = (g: IdleWeaponGroup): RailItem => ({
    key: g.key,
    icon: `${g.rarity}★`,
    title: g.name,
    sub: `${weaponTypeLabel(t, g.type)} · ${t.insights.actionQueue.idleSub}`,
    val: `×${g.count}`,
    tag: g.maxLevel === 1 ? t.insights.actionQueue.neverAscended : t.insights.actionQueue.stash,
    alert: g.maxLevel === 1,
  });

  const gemItems = data.hiddenGems.map((a) => artQualityItem(a, t.insights.actionQueue.gemsSub));
  const fodderItems = data.fodderCandidates.map((a) => artQualityItem(a, t.insights.actionQueue.fodderSub));
  const idleItems = data.idleWeapons.slice(0, 10).map(idleItem);
  const actionTotal = gemItems.length + fodderItems.length + idleItems.length;

  const toggleGroup = (k: string) => setOpenGroups((s) => ({ ...s, [k]: !s[k] }));

  const avgPct = Math.max(0, Math.min(100, o.avgInvestment));

  return (
    <div>
      <div className="ins-header">
        <span className="ins-header-title">{t.insights.title}</span>
        <span className="ins-header-sub">{t.insights.subtitle}</span>
      </div>

      <div className="ins-layout">
        <div className="ins-main">
          {/* hero */}
          <div className="ins-hero">
            <div className="card ins-ring-card">
              <div
                className="ins-ring"
                style={{ background: `conic-gradient(oklch(var(--acc)) 0 ${avgPct}%, var(--track) 0)` }}
              >
                <div className="ins-ring-inner">
                  <span className="ins-ring-value">{o.avgInvestment}</span>
                  <span className="ins-ring-unit">/ 100</span>
                </div>
              </div>
              <div className="ins-ring-label">
                <span className="card-label" style={{ marginBottom: 0 }}>
                  {t.insights.avgInvestment}
                </span>
                <span className="ins-ring-desc">{t.insights.avgInvestmentDesc}</span>
              </div>
            </div>
            <div className="ins-kpi-grid">
              {kpis.map((k) => (
                <Kpi key={k.label} label={k.label} value={k.value} sub={k.sub} />
              ))}
            </div>
          </div>

          {/* trio */}
          <div className="ins-trio">
            <div className="card">
              <div className="ins-card-head">
                <span className="card-label" style={{ marginBottom: 0 }}>
                  {t.insights.elements.title}
                </span>
                <span className="ins-card-meta">{t.insights.elements.charCount(o.characters)}</span>
              </div>
              <div className="ins-elem-bar">
                {data.elements.map((e) => (
                  <div
                    key={e.element}
                    style={{ flex: e.count, background: e.element in ELEM_COLOR ? `oklch(${ELEM_COLOR[e.element as Element]})` : 'var(--muted)' }}
                  />
                ))}
              </div>
              <div className="ins-elem-legend">
                {data.elements.map((e) => (
                  <div className="ins-elem-item" key={e.element}>
                    <span
                      className="ins-elem-tile"
                      style={
                        e.element in ELEM_COLOR
                          ? ({
                              '--el': ELEM_COLOR[e.element as Element],
                              background: 'oklch(var(--el) / .16)',
                              color: 'oklch(var(--el))',
                              borderColor: 'oklch(var(--el) / .5)',
                            } as CSSProperties)
                          : undefined
                      }
                    >
                      {elemCode(e.element)}
                    </span>
                    <span className="ins-elem-n">{e.count}</span>
                  </div>
                ))}
              </div>
            </div>

            <div className="card">
              <div className="ins-card-head">
                <span className="card-label" style={{ marginBottom: 0 }}>
                  {t.insights.weapons.title}
                </span>
                <div className="ins-wpn-legend">
                  <span>
                    <i style={{ background: 'var(--teal)' }} />
                    {t.insights.weapons.equipped}
                  </span>
                  <span>
                    <i style={{ border: '1px solid var(--border-strong)' }} />
                    {t.insights.weapons.stash}
                  </span>
                </div>
              </div>
              {weaponRows.map((w) => (
                <div className="ins-wpn-row" key={w.type}>
                  <span className="ins-wpn-type">{weaponTypeLabel(t, w.type)}</span>
                  <div className="ins-wpn-dots">
                    {Array.from({ length: w.eqDots + w.idleDots }, (_, i) => (
                      <span key={i} className={i < w.eqDots ? 'ins-wpn-dot filled' : 'ins-wpn-dot'} />
                    ))}
                  </div>
                  <span className="ins-wpn-count" style={w.heavyIdle ? { color: 'var(--red)' } : undefined}>
                    {w.equipped}
                    <span className="ins-wpn-count-total">/{w.total}</span>
                  </span>
                </div>
              ))}
            </div>

            <div className="card">
              <div className="ins-card-head">
                <span className="card-label" style={{ marginBottom: 0 }}>
                  {t.insights.rollQuality.title}
                </span>
                <span className="ins-card-meta">{t.insights.rollQuality.avg(Math.round(data.artifactQuality.avgRollQuality))}</span>
              </div>
              <div className="ins-rv-chart">
                {data.rollQualityBuckets.map((b, i) => {
                  const hi = i >= data.rollQualityBuckets.length - 2;
                  return (
                    <div className="ins-rv-col-wrap" key={b.label}>
                      <span className="ins-rv-n">{b.count}</span>
                      <div
                        className="ins-rv-col"
                        style={{
                          height: `${Math.max(2, (b.count / maxBucket) * 88)}%`,
                          background: hi ? 'oklch(var(--acc))' : `oklch(var(--acc) / ${0.25 + i * 0.15})`,
                          boxShadow: hi ? '0 0 14px oklch(var(--acc) / .4)' : 'none',
                        }}
                      />
                    </div>
                  );
                })}
              </div>
              <div className="ins-rv-labels">
                {data.rollQualityBuckets.map((b) => (
                  <span key={b.label}>{b.label}</span>
                ))}
              </div>
            </div>
          </div>

          {/* sets wall */}
          <div className="card">
            <div className="ins-card-head">
              <span className="card-label" style={{ marginBottom: 0 }}>
                {t.insights.sets.title}
              </span>
              <span className="ins-card-meta">{t.insights.sets.equippedPieces(setTotal)}</span>
            </div>
            <div className="ins-sets-grid">
              {topSets.map((s) => (
                <div className="ins-set-chip" key={s.key}>
                  <span className="ins-set-badge">{s.short}</span>
                  <div className="ins-set-info">
                    <div className="ins-set-name">{s.name}</div>
                    <div className="ins-set-share">{t.insights.sets.shareOfEquipped(pct(s.count, setTotal))}</div>
                  </div>
                  <span className="ins-set-n">{s.count}</span>
                </div>
              ))}
            </div>
          </div>

          {/* roster */}
          <div className="card">
            <div className="ins-card-head" style={{ marginBottom: 2 }}>
              <span className="card-label" style={{ marginBottom: 0 }}>
                {t.insights.roster.title}
              </span>
              <div className="ins-filter-row">
                {filterChips.map((f) => {
                  const active = filterEl === f.key;
                  const style =
                    f.key === 'All'
                      ? undefined
                      : ({ '--el': ELEM_COLOR[f.key as Element] } as CSSProperties);
                  return (
                    <div
                      key={f.key}
                      role="button"
                      className={'ins-filter-chip' + (active ? ' active' : '') + (f.key !== 'All' ? ' elem' : '')}
                      style={style}
                      onClick={() => setFilterEl(f.key)}
                    >
                      {f.label} <span className="ins-filter-n">{f.n}</span>
                    </div>
                  );
                })}
              </div>
            </div>
            <div className="ins-roster-grid">
              {rosterShown.map((c: CharacterInsight) => (
                <div
                  className="ins-roster-card clickable"
                  key={c.key}
                  style={elemStyle(c.element)}
                  role="button"
                  tabIndex={0}
                  onClick={() => setSelectedChar(c)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') setSelectedChar(c);
                  }}
                >
                  <div className="ins-roster-top">
                    <div className="ins-roster-avatar">
                      {c.name
                        .split(' ')
                        .map((w) => w[0])
                        .join('')
                        .slice(0, 2)
                        .toUpperCase()}
                    </div>
                    <div className="ins-roster-info">
                      <div className="ins-roster-name">{c.name}</div>
                      <div className="ins-roster-sub">{t.insights.roster.lvC(c.level, c.constellation)}</div>
                    </div>
                    <span className="ins-roster-score" style={{ color: tierColor(c.investment) }}>
                      {c.investment.toFixed(0)}
                    </span>
                  </div>
                  <div className="ins-roster-bar-track">
                    <div className="ins-roster-bar-fill" style={{ width: `${c.investment}%`, background: tierColor(c.investment) }} />
                  </div>
                  <div className="ins-roster-foot">
                    <span>{t.insights.roster.talentsAbbrev(c.avgTalent)}</span>
                    <span>{c.weaponName ?? '—'}</span>
                  </div>
                </div>
              ))}
            </div>
            {filtered.length > 15 && (
              <div role="button" className="ins-more-btn" onClick={() => setShowAllRoster((v) => !v)}>
                {showAllRoster ? t.insights.roster.showTop(15) : t.insights.roster.showAll(filtered.length)}
              </div>
            )}
          </div>
        </div>

        {/* action queue rail */}
        <div className="ins-rail">
          <div className="ins-rail-head">
            <span className="ins-rail-title">{t.insights.actionQueue.title}</span>
            <span className="ins-rail-total">{actionTotal}</span>
          </div>
          <RailGroup
            t={t}
            title={t.insights.actionQueue.gemsTitle}
            hint={t.insights.actionQueue.gemsHint}
            color="var(--green)"
            items={gemItems}
            open={!!openGroups.gems}
            onToggle={() => toggleGroup('gems')}
          />
          <RailGroup
            t={t}
            title={t.insights.actionQueue.fodderTitle}
            hint={t.insights.actionQueue.fodderHint}
            color="var(--red)"
            items={fodderItems}
            open={!!openGroups.fodder}
            onToggle={() => toggleGroup('fodder')}
          />
          <RailGroup
            t={t}
            title={t.insights.actionQueue.idleTitle}
            hint={t.insights.actionQueue.idleHint}
            color="var(--teal)"
            items={idleItems}
            open={!!openGroups.idle}
            onToggle={() => toggleGroup('idle')}
          />
        </div>
      </div>

      {selectedChar && <ScoreBreakdownModal character={selectedChar} onClose={() => setSelectedChar(null)} />}
    </div>
  );
}
