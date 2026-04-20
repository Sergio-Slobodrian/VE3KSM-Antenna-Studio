// Copyright 2026 Sergio Slobodrian
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
