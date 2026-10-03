import { useState } from 'react';
import { ACCENTS, Accent, TOPN_OPTS } from '../lib/refdata';
import { LANGS, useLanguage, type Lang } from '../i18n';

export type View = 'import' | 'configure' | 'results' | 'insights';

interface Props {
  view: View;
  onNavigate: (v: View) => void;
  canConfigure: boolean;
  canResults: boolean;
  canInsights: boolean;
  accent: Accent;
  onAccentChange: (a: Accent) => void;
  showSolverStats: boolean;
  onShowSolverStatsChange: (v: boolean) => void;
  defaultTopN: number;
  onDefaultTopNChange: (n: number) => void;
}

const LANG_LABEL: Record<Lang, string> = { en: 'EN', fr: 'FR' };

export default function TopBar(props: Props) {
  const { t, lang, setLang } = useLanguage();
  const [settingsOpen, setSettingsOpen] = useState(false);

  const STEPS: { view: View; label: string }[] = [
    { view: 'import', label: t.topbar.steps.import },
    { view: 'configure', label: t.topbar.steps.configure },
    { view: 'results', label: t.topbar.steps.results },
    { view: 'insights', label: t.topbar.steps.insights },
  ];

  const disabledFor = (v: View) => {
    if (v === 'configure') return !props.canConfigure;
    if (v === 'results') return !props.canResults;
    if (v === 'insights') return !props.canInsights;
    return false;
  };

  return (
    <div className="topbar">
      <div className="topbar-left">
        <div className="logo-mark">◆</div>
        <span className="wordmark">ARTIFACT OPTIMIZER</span>
        <span className="badge">GOOD · v2</span>
      </div>
      <div className="topbar-right" style={{ position: 'relative' }}>
        {STEPS.map((s, i) => (
          <button
            key={s.view}
            className={'step' + (props.view === s.view ? ' active' : '')}
            disabled={disabledFor(s.view)}
            onClick={() => props.onNavigate(s.view)}
          >
            0{i + 1} {s.label}
          </button>
        ))}
        <button
          className="settings-btn"
          title={t.topbar.displaySettings}
          onClick={() => setSettingsOpen((v) => !v)}
        >
          ⚙
        </button>
        {settingsOpen && (
          <div className="settings-panel">
            <div className="settings-label">{t.topbar.language}</div>
            <div className="chip-row">
              {LANGS.map((l) => (
                <button
                  key={l}
                  className={'chip' + (lang === l ? ' active' : '')}
                  onClick={() => setLang(l)}
                >
                  {LANG_LABEL[l]}
                </button>
              ))}
            </div>
            <div className="settings-label">{t.topbar.accent}</div>
            <div className="swatch-row">
              {ACCENTS.map((a) => (
                <button
                  key={a}
                  className={'swatch accent-' + a + (props.accent === a ? ' active' : '')}
                  style={{ background: 'oklch(var(--acc-lt))' }}
                  onClick={() => props.onAccentChange(a)}
                  title={a}
                />
              ))}
            </div>
            <div className="settings-label">{t.topbar.solverStats}</div>
            <label className="settings-toggle">
              <span>{t.topbar.showInResults}</span>
              <input
                type="checkbox"
                checked={props.showSolverStats}
                onChange={(e) => props.onShowSolverStatsChange(e.target.checked)}
              />
            </label>
            <div className="settings-label">{t.topbar.defaultTopN}</div>
            <div className="chip-row">
              {TOPN_OPTS.map((n) => (
                <button
                  key={n}
                  className={'chip' + (props.defaultTopN === n ? ' active' : '')}
                  onClick={() => props.onDefaultTopNChange(n)}
                >
                  {n}
                </button>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
