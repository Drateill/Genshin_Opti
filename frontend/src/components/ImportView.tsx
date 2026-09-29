import { useRef, useState } from 'react';
import type { ImportSummary } from '../api/types';
import { useT } from '../i18n';

interface Props {
  rawText: string;
  onRawTextChange: (v: string) => void;
  importing: boolean;
  importError: string | null;
  summary: ImportSummary | null;
  onParseImport: () => void;
  onLoadSample: () => void;
  onDropFile: (file: File) => void;
  onContinue: () => void;
}

export default function ImportView(props: Props) {
  const t = useT();
  const [dragging, setDragging] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  return (
    <div className="fade">
      <div style={{ marginBottom: 24 }}>
        <div className="page-title">{t.import.title}</div>
        <div className="page-sub">{t.import.subtitle}</div>
      </div>

      <details className="accordion">
        <summary>{t.import.tutorialToggle}</summary>
        <div className="accordion-body">
          <p className="import-tutorial-intro">{t.import.tutorialIntro}</p>
          <ol className="import-tutorial-steps">
            {t.import.tutorialSteps.map((step, i) => (
              <li key={i}>{step}</li>
            ))}
          </ol>
          <div className="import-tutorial-links">
            <a href="https://github.com/konkers/irminsul/releases" target="_blank" rel="noopener noreferrer">
              {t.import.tutorialReleasesLink}
            </a>
            <a href="https://konkers.github.io/irminsul/02-quickstart.html" target="_blank" rel="noopener noreferrer">
              {t.import.tutorialGuideLink}
            </a>
          </div>
        </div>
      </details>

      <div className="import-grid">
        <div className="card">
          <div
            className={'dropzone' + (dragging ? ' dragging' : '')}
            onDragOver={(e) => {
              e.preventDefault();
              setDragging(true);
            }}
            onDragLeave={() => setDragging(false)}
            onDrop={(e) => {
              e.preventDefault();
              setDragging(false);
              const file = e.dataTransfer.files?.[0];
              if (file) props.onDropFile(file);
            }}
            onClick={() => fileInputRef.current?.click()}
            role="button"
            tabIndex={0}
          >
            <div className="dropzone-icon">↥</div>
            <div className="dropzone-title">{t.import.dropTitle('good.json')}</div>
            <div className="dropzone-sub">{t.import.dropSub}</div>
            <input
              ref={fileInputRef}
              type="file"
              accept="application/json,.json"
              hidden
              onChange={(e) => {
                const file = e.target.files?.[0];
                if (file) props.onDropFile(file);
                e.target.value = '';
              }}
            />
          </div>
          <textarea
            className="import-textarea"
            placeholder='{ "format": "GOOD", "version": 2, "characters": [...], "artifacts": [...] }'
            value={props.rawText}
            onChange={(e) => props.onRawTextChange(e.target.value)}
          />
          <div className="import-actions">
            <button
              className="btn btn-primary"
              disabled={props.importing || !props.rawText.trim()}
              onClick={props.onParseImport}
            >
              {props.importing ? t.import.parsing : t.import.parseImport}
            </button>
            <button className="btn btn-secondary" disabled={props.importing} onClick={props.onLoadSample}>
              {t.import.loadSample}
            </button>
          </div>
          {props.importError && <div className="import-error">{props.importError}</div>}
        </div>

        <div className="card">
          {!props.summary ? (
            <div className="summary-empty">
              {t.import.noInventory}
              <br />
              {t.import.importToSeeSummary}
            </div>
          ) : (
            <div className="fade">
              <div className="card-label">{t.import.summaryTitle}</div>
              <div className="summary-list">
                <div className="summary-row">
                  <span className="summary-row-label">{t.import.characters}</span>
                  <span className="summary-row-value">{props.summary.characters}</span>
                </div>
                <div className="summary-row">
                  <span className="summary-row-label">{t.import.artifacts}</span>
                  <span className="summary-row-value">{props.summary.artifacts}</span>
                </div>
                <div className="summary-row">
                  <span className="summary-row-label">{t.import.setsDetected}</span>
                  <span className="summary-row-value">{props.summary.sets}</span>
                </div>
              </div>
              <button className="btn btn-primary" style={{ width: '100%' }} onClick={props.onContinue}>
                {t.import.continue}
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
