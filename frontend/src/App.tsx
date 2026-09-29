import { useEffect, useRef, useState } from 'react';
import TopBar, { View } from './components/TopBar';
import ImportView from './components/ImportView';
import ConfigureView from './components/ConfigureView';
import ResultsView from './components/ResultsView';
import { api, ApiError } from './api/client';
import type {
  GoodExport,
  ImportSummary,
  RosterEntry,
  SetInfo,
  SolveResponse,
  StatRange,
  WeaponOption,
} from './api/types';
import { Accent } from './lib/refdata';
import { loadPersisted, savePersisted, type PersistedState } from './lib/persist';
import { localeTag, useLanguage, type Lang } from './i18n';

export default function App() {
  const { lang, setLang, t } = useLanguage();
  const [view, setView] = useState<View>('import');
  const [accent, setAccent] = useState<Accent>('orange');
  const [showSolverStats, setShowSolverStats] = useState(true);
  const [defaultTopN, setDefaultTopN] = useState(5);

  // import
  const [rawText, setRawText] = useState('');
  const [importing, setImporting] = useState(false);
  const [importError, setImportError] = useState<string | null>(null);
  const [summary, setSummary] = useState<ImportSummary | null>(null);
  const [roster, setRoster] = useState<RosterEntry[]>([]);
  const [sets, setSets] = useState<SetInfo[]>([]);

  // configure
  const [charKey, setCharKey] = useState('');
  const [targetSetKey, setTargetSetKey] = useState('');
  const [targetSetKey2, setTargetSetKey2] = useState(''); // dual-set mode's second pick, empty in single-set mode
  const [dualSetMode, setDualSetMode] = useState(false);
  const [sands, setSands] = useState<string[]>(['em', 'atk_']);
  const [goblet, setGoblet] = useState<string[]>([]);
  const [circlet, setCirclet] = useState<string[]>(['critRate_', 'critDMG_']);
  const [constraints, setConstraints] = useState<Record<string, StatRange>>({ critRate_: { min: 70 } });
  const [topN, setTopN] = useState(defaultTopN);
  const [includeEquippedByOthers, setIncludeEquippedByOthers] = useState(true);
  const [weaponOptions, setWeaponOptions] = useState<WeaponOption[]>([]);
  const [weaponId, setWeaponId] = useState<number | undefined>(undefined);

  // solve
  const [solving, setSolving] = useState(false);
  const [solveError, setSolveError] = useState<string | null>(null);
  const [results, setResults] = useState<SolveResponse | null>(null);
  const [selIdx, setSelIdx] = useState(0);
  const [solveProgress, setSolveProgress] = useState<{ tested: number; total: number; elapsedMs: number } | null>(
    null
  );

  const restoring = useRef(false);
  useEffect(() => {
    if (restoring.current) return;
    restoring.current = true;
    const saved = loadPersisted();
    const initialLang: Lang = saved.lang ?? 'en';
    if (saved.lang) setLang(saved.lang);
    if (!saved.rawText) return;
    setRawText(saved.rawText);
    setAccent(saved.accent ?? 'orange');
    setShowSolverStats(saved.showSolverStats ?? true);
    setDefaultTopN(saved.defaultTopN ?? 5);
    setIncludeEquippedByOthers(saved.includeEquippedByOthers ?? true);
    importRaw(saved.rawText, saved, initialLang).then((ok) => {
      if (ok) setView(saved.view === 'results' ? 'configure' : saved.view ?? 'configure');
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // A language switch after the initial import needs the already-fetched
  // roster/sets/weapon names re-fetched in the new language — the raw
  // import data doesn't change, only how chardb's names are rendered.
  const langMounted = useRef(false);
  useEffect(() => {
    if (!langMounted.current) {
      langMounted.current = true;
      return;
    }
    if (!summary) return;
    (async () => {
      const [rosterList, setList] = await Promise.all([api.characters(lang), api.sets(lang)]);
      setRoster(rosterList);
      setSets(setList);
      const current = rosterList.find((r) => r.key === charKey);
      if (current?.weaponType) await loadWeaponsForType(current.weaponType, weaponId, lang);
    })();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [lang]);

  useEffect(() => {
    if (!rawText) return; // nothing imported yet — don't persist an empty session
    savePersisted({
      rawText, view, charKey, targetSetKey, targetSetKey2: dualSetMode ? targetSetKey2 : undefined,
      weaponId, sands, goblet, circlet, constraints, topN,
      accent, showSolverStats, defaultTopN, includeEquippedByOthers, lang,
    });
  }, [rawText, view, charKey, targetSetKey, targetSetKey2, dualSetMode, weaponId, sands, goblet, circlet, constraints, topN, accent, showSolverStats, defaultTopN, includeEquippedByOthers, lang]);

  const toggleIn = (list: string[], setter: (v: string[]) => void, val: string) => {
    setter(list.includes(val) ? list.filter((x) => x !== val) : [...list, val]);
  };

  function setConstraint(key: string, field: 'min' | 'max', value: number | undefined) {
    setConstraints((prev) => {
      const range = { ...prev[key], [field]: value };
      if (range.min === undefined && range.max === undefined) {
        const next = { ...prev };
        delete next[key];
        return next;
      }
      return { ...prev, [key]: range };
    });
  }

  async function loadWeaponsForType(weaponType: string | undefined, preferredId?: number, langOverride?: Lang) {
    if (!weaponType) {
      setWeaponOptions([]);
      setWeaponId(undefined);
      return;
    }
    try {
      const list = await api.weapons(weaponType, langOverride ?? lang);
      setWeaponOptions(list);
      const preferred = preferredId !== undefined && list.some((w) => w.id === preferredId) ? preferredId : list[0]?.id;
      setWeaponId(preferred);
    } catch {
      setWeaponOptions([]);
      setWeaponId(undefined);
    }
  }

  async function afterImport(s: ImportSummary, restore?: PersistedState, langOverride?: Lang) {
    const useLang = langOverride ?? lang;
    setSummary(s);
    const [rosterList, setList] = await Promise.all([api.characters(useLang), api.sets(useLang)]);
    setRoster(rosterList);
    setSets(setList);
    const restoredChar = restore?.charKey ? rosterList.find((r) => r.key === restore.charKey) : undefined;
    const known = restoredChar ?? rosterList.find((r) => r.known) ?? rosterList[0];
    if (known) {
      setCharKey(known.key);
      const restoredSet = restore?.targetSetKey && setList.some((x) => x.key === restore.targetSetKey)
        ? restore.targetSetKey
        : undefined;
      const hasRecSet = setList.some((x) => x.key === known.recSet);
      setTargetSetKey(restoredSet ?? (hasRecSet ? known.recSet : setList[0]?.key ?? ''));
      const restoredSet2 = restore?.targetSetKey2 && setList.some((x) => x.key === restore.targetSetKey2)
        ? restore.targetSetKey2
        : undefined;
      setTargetSetKey2(restoredSet2 ?? '');
      setDualSetMode(!!restoredSet2);
      setGoblet(restore?.goblet ?? (known.dmgKey ? [known.dmgKey] : []));
      await loadWeaponsForType(known.weaponType, restore?.weaponId, useLang);
    } else {
      setCharKey('');
      setTargetSetKey(setList[0]?.key ?? '');
      setTargetSetKey2('');
      setDualSetMode(false);
    }
    if (restore?.sands) setSands(restore.sands);
    if (restore?.circlet) setCirclet(restore.circlet);
    if (restore?.constraints) setConstraints(restore.constraints);
    if (restore?.topN) setTopN(restore.topN);
  }

  async function importRaw(text: string, restore?: PersistedState, langOverride?: Lang): Promise<boolean> {
    setImporting(true);
    setImportError(null);
    try {
      const s = await api.importGood(text);
      await afterImport(s, restore, langOverride);
      return true;
    } catch (e) {
      setImportError(e instanceof ApiError ? e.message : t.import.importFailed);
      return false;
    } finally {
      setImporting(false);
    }
  }

  async function importData(data: GoodExport) {
    setImporting(true);
    setImportError(null);
    try {
      const s = await api.importExport(data);
      await afterImport(s);
    } catch (e) {
      setImportError(e instanceof ApiError ? e.message : t.import.importFailed);
    } finally {
      setImporting(false);
    }
  }

  async function loadSample() {
    setImporting(true);
    setImportError(null);
    try {
      const data = await api.fetchSample();
      setRawText(JSON.stringify(data, null, 2));
      await importData(data);
    } catch (e) {
      setImportError(e instanceof ApiError ? e.message : t.import.sampleFailed);
    } finally {
      setImporting(false);
    }
  }

  function onDropFile(file: File) {
    const reader = new FileReader();
    reader.onload = () => {
      const text = String(reader.result ?? '');
      setRawText(text);
      importRaw(text);
    };
    reader.readAsText(file);
  }

  async function selectChar(key: string) {
    setCharKey(key);
    const r = roster.find((x) => x.key === key);
    if (r?.known) {
      const hasRecSet = sets.some((s) => s.key === r.recSet);
      if (hasRecSet) setTargetSetKey(r.recSet);
      setGoblet(r.dmgKey ? [r.dmgKey] : []);
    }
    // A newly-picked character has no reason to inherit a stale 2nd-set pick.
    setTargetSetKey2('');
    setDualSetMode(false);
    await loadWeaponsForType(r?.weaponType);
  }

  // onSelectSet holds all the set-picker click semantics so ConfigureView
  // just forwards clicks. In single-set mode it behaves exactly like the
  // plain setter it replaces. In dual-set mode: clicking either current pick
  // deselects it (promoting the other to primary if needed); clicking a new
  // set fills whichever slot is empty; with both slots full, the oldest
  // (primary) pick is evicted and the newest becomes the secondary — a
  // simple 2-slot sliding window, so every click does something.
  function onSelectSet(key: string) {
    if (!dualSetMode) {
      setTargetSetKey(key);
      setTargetSetKey2('');
      return;
    }
    if (key === targetSetKey) {
      setTargetSetKey(targetSetKey2);
      setTargetSetKey2('');
      return;
    }
    if (key === targetSetKey2) {
      setTargetSetKey2('');
      return;
    }
    if (!targetSetKey) {
      setTargetSetKey(key);
      return;
    }
    if (!targetSetKey2) {
      setTargetSetKey2(key);
      return;
    }
    setTargetSetKey(targetSetKey2);
    setTargetSetKey2(key);
  }

  function onToggleDualSetMode() {
    setDualSetMode((v) => {
      if (v) setTargetSetKey2('');
      return !v;
    });
  }

  async function runSolve() {
    setSolving(true);
    setSolveError(null);
    const start = performance.now();
    setSolveProgress({ tested: 0, total: 0, elapsedMs: 0 });
    try {
      const { jobId } = await api.solveStart({
        characterKey: charKey,
        targetSetKey,
        targetSetKey2: dualSetMode ? targetSetKey2 : undefined,
        weaponId,
        slotConstraints: { sands, goblet, circlet },
        constraints,
        objective: 'critValue',
        topN,
        includeEquippedByOthers,
        lang,
      });
      for (;;) {
        const p = await api.solveProgress(jobId);
        setSolveProgress({ tested: p.tested, total: p.total, elapsedMs: performance.now() - start });
        if (p.done) {
          if (p.error) throw new ApiError(p.error);
          setResults(p.result ?? null);
          setSelIdx(0);
          setView('results');
          break;
        }
        await new Promise((r) => setTimeout(r, 300));
      }
    } catch (e) {
      setSolveError(e instanceof ApiError ? e.message : t.errors.solveFailed);
    } finally {
      setSolving(false);
      setSolveProgress(null);
    }
  }

  return (
    <div className={'app accent-' + accent}>
      <TopBar
        view={view}
        onNavigate={setView}
        canConfigure={!!summary}
        canResults={!!results}
        accent={accent}
        onAccentChange={setAccent}
        showSolverStats={showSolverStats}
        onShowSolverStatsChange={setShowSolverStats}
        defaultTopN={defaultTopN}
        onDefaultTopNChange={(n) => {
          setDefaultTopN(n);
          setTopN(n);
        }}
      />

      <div className="container">
        {view === 'import' && (
          <ImportView
            rawText={rawText}
            onRawTextChange={setRawText}
            importing={importing}
            importError={importError}
            summary={summary}
            onParseImport={() => importRaw(rawText)}
            onLoadSample={loadSample}
            onDropFile={onDropFile}
            onContinue={() => setView('configure')}
          />
        )}

        {view === 'configure' && (
          <ConfigureView
            roster={roster}
            sets={sets}
            charKey={charKey}
            targetSetKey={targetSetKey}
            targetSetKey2={targetSetKey2}
            dualSetMode={dualSetMode}
            sands={sands}
            goblet={goblet}
            circlet={circlet}
            constraints={constraints}
            topN={topN}
            weaponOptions={weaponOptions}
            weaponId={weaponId}
            onSelectWeapon={setWeaponId}
            onSelectChar={selectChar}
            onSelectSet={onSelectSet}
            onToggleDualSetMode={onToggleDualSetMode}
            onToggleSands={(k) => toggleIn(sands, setSands, k)}
            onToggleGoblet={(k) => toggleIn(goblet, setGoblet, k)}
            onToggleCirclet={(k) => toggleIn(circlet, setCirclet, k)}
            onSetConstraint={setConstraint}
            onSetTopN={setTopN}
            includeEquippedByOthers={includeEquippedByOthers}
            onToggleIncludeEquippedByOthers={setIncludeEquippedByOthers}
            onSolve={runSolve}
            solving={solving}
          />
        )}

        {view === 'results' && (
          <ResultsView
            results={results}
            solveError={solveError}
            targetSetKey={targetSetKey}
            targetSetKey2={dualSetMode ? targetSetKey2 : undefined}
            sets={sets}
            roster={roster}
            charKey={charKey}
            weapon={weaponOptions.find((w) => w.id === weaponId)}
            selIdx={selIdx}
            onSelectRank={setSelIdx}
            onBackToConfigure={() => setView('configure')}
            showSolverStats={showSolverStats}
          />
        )}

        {solving && (
          <div className="solving-overlay">
            <div className="spinner" />
            <div className="solving-label">
              {solveProgress && solveProgress.tested > 0
                ? t.solving.combinationsTested(solveProgress.tested.toLocaleString(localeTag(lang)))
                : t.solving.starting}
            </div>
            {solveProgress && (
              <>
                <div className="solving-progress-track">
                  <div
                    className="solving-progress-fill"
                    style={{
                      width:
                        solveProgress.total > 0
                          ? `${Math.max(2, Math.min(100, (solveProgress.tested / solveProgress.total) * 100))}%`
                          : '2%',
                    }}
                  />
                </div>
                <div className="solving-sub">
                  {t.solving.elapsed((solveProgress.elapsedMs / 1000).toFixed(1))}
                  {solveProgress.total > 0 && t.solving.toCheck(solveProgress.total.toLocaleString(localeTag(lang)))}
                </div>
              </>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
