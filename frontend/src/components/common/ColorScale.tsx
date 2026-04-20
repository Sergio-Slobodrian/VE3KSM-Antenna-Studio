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
 * Vertical colour-scale legend for the radiation pattern viewer.
 *
 * Renders a CSS linear-gradient bar (red at top / high gain, blue at bottom /
 * low gain) with labelled min, midpoint, and max values.  Designed to be
 * overlaid on the 3D canvas as an absolute-positioned element.
 */
import React from 'react';

interface ColorScaleProps {
  minValue: number;
  maxValue: number;
  /** Label shown above the gradient bar (default: "Gain (dBi)"). */
  label?: string;
}

const ColorScale: React.FC<ColorScaleProps> = ({ minValue, maxValue, label = 'Gain (dBi)' }) => {
  const gradientStyle: React.CSSProperties = {
    width: '20px',
    height: '150px',
    background: 'linear-gradient(to bottom, #ff0000, #ffff00, #00ff00, #0088ff, #0000ff)',
    borderRadius: '3px',
    border: '1px solid #555',
  };

  return (
    <div className="color-scale">
      <div className="color-scale-label">{label}</div>
      <div style={{ display: 'flex', alignItems: 'stretch', gap: '6px' }}>
        <div style={gradientStyle} />
        <div
          style={{
            display: 'flex',
            flexDirection: 'column',
            justifyContent: 'space-between',
            fontSize: '11px',
            color: '#ccc',
          }}
        >
          <span>{maxValue.toFixed(1)}</span>
          <span>{((maxValue + minValue) / 2).toFixed(1)}</span>
          <span>{minValue.toFixed(1)}</span>
        </div>
      </div>
    </div>
  );
};

export default ColorScale;
