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
 * SWR vs Frequency chart (Recharts line chart).
 *
 * Features:
 *  - Automatic log-scale switching when the SWR range exceeds 10:1.
 *    In log mode the Y axis plots log10(SWR) with custom tick labels that
 *    show the original SWR values (e.g. 1, 2, 5, 10, 100).
 *  - Reference lines at SWR 2:1 (orange dashed) and 3:1 (grey dashed).
 *  - Tooltip shows the raw SWR value regardless of scale mode.
 *  - SWR values below 1 are clamped to 1 in the log calculation to avoid
 *    negative log results from numerical noise.
 */
import React, { useMemo } from 'react';
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ReferenceLine,
  ResponsiveContainer,
} from 'recharts';
import { useAntennaStore } from '@/store/antennaStore';

const SWRChart: React.FC = () => {
  const sweepResult = useAntennaStore((s) => s.sweepResult);

  // Prepare chart data: include both raw SWR and log10(SWR) so either can be plotted
  const data = useMemo(() => {
    if (!sweepResult) return [];
    return sweepResult.frequencies.map((freq, i) => {
      const raw = sweepResult.swr[i];
      return {
        frequency: freq,
        swr: raw,
        swrLog: Math.log10(Math.max(raw, 1)), // clamp to 1 to avoid log of values < 1
      };
    });
  }, [sweepResult]);

  const { minSwr, maxSwr } = useMemo(() => {
    if (data.length === 0) return { minSwr: 1, maxSwr: 10 };
    let mn = Infinity, mx = 0;
    for (const d of data) {
      if (d.swr < mn) mn = d.swr;
      if (d.swr > mx) mx = d.swr;
    }
    return { minSwr: mn, maxSwr: mx };
  }, [data]);

  if (!sweepResult || data.length === 0) {
    return (
      <div className="no-data-message">
        <p>No SWR data.</p>
        <p>Run a frequency sweep to see the SWR chart.</p>
      </div>
    );
  }

  // Automatically switch to log scale when the SWR dynamic range exceeds 10:1
  const useLog = maxSwr / Math.max(minSwr, 1) > 10;

  // Predefined "nice" SWR tick values; filter to the visible range for log-scale ticks
  const logTicks = [1, 2, 3, 5, 10, 20, 50, 100, 500, 1000, 10000];
  const logTicksFiltered = logTicks.filter(
    (t) => t >= Math.floor(minSwr) && t <= Math.min(maxSwr * 1.2, 100000)
  );

  return (
    <div className="chart-container">
      <h3>
        SWR vs Frequency
        {useLog && (
          <span style={{ fontSize: '11px', color: '#aaa', marginLeft: 12, fontWeight: 400 }}>
            (log scale)
          </span>
        )}
      </h3>
      <ResponsiveContainer width="100%" height={400}>
        <LineChart data={data} margin={{ top: 10, right: 30, left: 10, bottom: 10 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="#333" />
          <XAxis
            dataKey="frequency"
            stroke="#aaa"
            label={{ value: 'Frequency (MHz)', position: 'insideBottom', offset: -5, fill: '#aaa' }}
          />
          {useLog ? (
            <YAxis
              dataKey="swrLog"
              stroke="#aaa"
              domain={[0, 'auto']}
              ticks={logTicksFiltered.map((t) => Math.log10(t))}
              tickFormatter={(v: number) => {
                const val = Math.pow(10, v);
                if (val >= 1000) return `${(val / 1000).toFixed(0)}k`;
                if (val >= 100) return val.toFixed(0);
                return val.toFixed(val < 10 ? 1 : 0);
              }}
              label={{ value: 'SWR', angle: -90, position: 'insideLeft', fill: '#aaa' }}
            />
          ) : (
            <YAxis
              dataKey="swr"
              domain={[1, Math.ceil(Math.min(maxSwr * 1.1, 50))]}
              stroke="#aaa"
              label={{ value: 'SWR', angle: -90, position: 'insideLeft', fill: '#aaa' }}
            />
          )}
          <Tooltip
            contentStyle={{ backgroundColor: '#2a2a3e', border: '1px solid #555', color: '#eee' }}
            formatter={(_: unknown, name: string, entry: { payload: { swr: number } }) => {
              const raw = entry.payload.swr;
              const display = raw >= 999 ? `${raw.toFixed(0)}` : raw.toFixed(2);
              return [`${display} : 1`, 'SWR'];
            }}
            labelFormatter={(label: number) => `${label.toFixed(3)} MHz`}
          />
          <ReferenceLine
            y={useLog ? Math.log10(2) : 2}
            stroke="#ff8844"
            strokeDasharray="5 5"
            label={{ value: '2:1', fill: '#ff8844', fontSize: 11 }}
          />
          <ReferenceLine
            y={useLog ? Math.log10(3) : 3}
            stroke="#666"
            strokeDasharray="2 4"
          />
          <Line
            type="monotone"
            dataKey={useLog ? 'swrLog' : 'swr'}
            stroke="#44aaff"
            strokeWidth={2}
            dot={false}
            activeDot={{ r: 4 }}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
};

export default SWRChart;
