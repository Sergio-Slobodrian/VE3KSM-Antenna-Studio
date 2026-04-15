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
