/**
 * Export controls for the current sweep result.  Renders two buttons
 * that download the sweep as a CSV or a Touchstone .s1p file.  Hidden
 * when no sweep has been run.
 */
import React from 'react';
import { useAntennaStore } from '@/store/antennaStore';
import { sweepToCSV, sweepToTouchstone, downloadText } from '@/utils/sweepExport';

const SweepExport: React.FC = () => {
  const { sweepResult } = useAntennaStore();
  if (!sweepResult || sweepResult.frequencies.length === 0) return null;

  const onCSV = () => {
    downloadText('sweep.csv', sweepToCSV(sweepResult), 'text/csv');
  };
  const onS1P = () => {
    downloadText('sweep.s1p', sweepToTouchstone(sweepResult), 'application/octet-stream');
  };

  return (
    <div className="sweep-export">
      <button className="btn btn-outline btn-small" onClick={onCSV} title="Frequency / Z / SWR / Γ as CSV">
        ⬇ CSV
      </button>
      <button className="btn btn-outline btn-small" onClick={onS1P} title="Touchstone .s1p (S-parameters) for VNA tools">
        ⬇ .s1p
      </button>
    </div>
  );
};

export default SweepExport;
