/**
 * WarningsBanner renders the non-blocking accuracy warnings produced by
 * the MoM segmentation validator.  Errors are shown in red, warnings in
 * amber, info in blue.  Each row points at the offending wire when
 * applicable.
 */
import React, { useState } from 'react';
import { useAntennaStore } from '@/store/antennaStore';
import type { Warning } from '@/types';

const sevColor: Record<Warning['severity'], string> = {
  info: 'var(--color-info, #3b82f6)',
  warn: 'var(--color-warn, #f59e0b)',
  error: 'var(--color-error, #dc2626)',
};

const WarningsBanner: React.FC = () => {
  const {
    simulationResult,
    sweepResult,
    simulationResultSeq,
    sweepResultSeq,
  } = useAntennaStore();
  const [collapsed, setCollapsed] = useState(false);
  // Show whichever result was produced most recently.  Sweeps don't
  // populate simulationResult and vice versa, so without this check
  // the prior simulate's warnings persist after a successful sweep.
  // Pick whichever result was set with the higher monotonic seq.
  // Ties (both at 0 — fresh page) yield an empty list.
  let warnings: Warning[] = [];
  if (sweepResultSeq === 0 && simulationResultSeq === 0) {
    warnings = [];
  } else if (sweepResultSeq >= simulationResultSeq) {
    warnings = sweepResult?.warnings ?? [];
  } else {
    warnings = simulationResult?.warnings ?? [];
  }
  if (warnings.length === 0) return null;

  const errors = warnings.filter((w) => w.severity === 'error').length;
  const warns = warnings.filter((w) => w.severity === 'warn').length;

  return (
    <div className="warnings-banner">
      <button
        className="warnings-summary"
        onClick={() => setCollapsed((c) => !c)}
        title="Click to expand / collapse"
      >
        <span style={{ color: sevColor.error }}>{errors} error{errors === 1 ? '' : 's'}</span>
        {' • '}
        <span style={{ color: sevColor.warn }}>{warns} warning{warns === 1 ? '' : 's'}</span>
        {' '}
        <span className="muted small">{collapsed ? '(show)' : '(hide)'}</span>
      </button>
      {!collapsed && (
        <ul className="warnings-list">
          {warnings.map((w, i) => (
            <li key={i} className="warning-row">
              <span
                className="warning-severity"
                style={{ color: sevColor[w.severity] }}
              >
                {w.severity.toUpperCase()}
              </span>
              <span className="warning-code">{w.code}</span>
              {w.wireIndex !== undefined && w.wireIndex >= 0 && (
                <span className="warning-loc">wire {w.wireIndex + 1}</span>
              )}
              <span className="warning-message">{w.message}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
};

export default WarningsBanner;
