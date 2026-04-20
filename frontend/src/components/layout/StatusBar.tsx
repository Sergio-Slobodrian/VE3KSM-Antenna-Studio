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
 * Bottom status bar.
 *
 * Displays one of four states:
 *  - Error message (red) when a simulation fails or validation rejects input.
 *  - "Simulating..." spinner while a backend request is in flight.
 *  - Result summary (impedance, SWR, gain) after a successful simulation.
 *  - Idle "Ready" message otherwise.
 */
import React from 'react';
import { useAntennaStore } from '@/store/antennaStore';
import { formatImpedance } from '@/utils/conversions';

const StatusBar: React.FC = () => {
  const { simulationResult, isSimulating, error } = useAntennaStore();

  let content: React.ReactNode;

  if (error) {
    content = <span className="status-error">{error}</span>;
  } else if (isSimulating) {
    content = <span className="status-running">Simulating...</span>;
  } else if (simulationResult) {
    const { impedance, swr, gainDbi } = simulationResult;
    content = (
      <span className="status-result">
        Z = {formatImpedance(impedance.r, impedance.x)} | SWR = {swr.toFixed(2)} | Gain ={' '}
        {gainDbi.toFixed(2)} dBi
      </span>
    );
  } else {
    content = <span className="status-idle">Ready. Configure antenna and run simulation.</span>;
  }

  return <div className="status-bar">{content}</div>;
};

export default StatusBar;
