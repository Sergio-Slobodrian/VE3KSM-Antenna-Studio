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
 * Editable row for a single wire in the WireTable.
 *
 * Handles unit conversion between the internal meters representation and the
 * user-selected display unit.  Length fields (coordinates + radius) are
 * multiplied by METERS_TO_UNIT[displayUnit] for display and divided back on
 * input.  The 'segments' field is unitless and passed through directly.
 *
 * Adds a Material dropdown for the new conductor-loss feature.
 */
import React from 'react';
import type { Wire, Material } from '@/types';
import { METERS_TO_UNIT, MATERIAL_LABELS, COATING_PRESETS, UNIT_LABELS } from '@/types';
import { useAntennaStore } from '@/store/antennaStore';

interface WireRowProps {
  wire: Wire;
  index: number;
}

/** Fields that represent lengths/positions and should be unit-converted. */
const lengthFields = new Set<keyof Wire>([
  'x1', 'y1', 'z1', 'x2', 'y2', 'z2', 'radius',
  'radiusStart', 'radiusEnd', 'coatingThickness',
]);

const numericFields: (keyof Wire)[] = ['x1', 'y1', 'z1', 'x2', 'y2', 'z2', 'radius', 'segments'];
const materialOptions: Material[] = ['', 'copper', 'aluminum', 'brass', 'steel', 'stainless', 'silver', 'gold'];

const WireRow: React.FC<WireRowProps> = ({ wire, index }) => {
  const { updateWire, removeWire, selectWire, selectedWireId, displayUnit } = useAntennaStore();
  const isSelected = selectedWireId === wire.id;
  const factor = METERS_TO_UNIT[displayUnit];

  const [coatingPreset, setCoatingPreset] = React.useState('bare');
  const [expanded, setExpanded] = React.useState(false);

  const applyCoatingPreset = (key: string) => {
    setCoatingPreset(key);
    if (key === '') return;
    const p = COATING_PRESETS.find((cp) => cp.key === key);
    if (!p) return;
    updateWire(wire.id, {
      coatingThickness: p.thickness,
      coatingEpsR: p.epsR,
      coatingLossTan: p.lossTan,
    });
  };

  const handleNumChange = (field: keyof Wire, value: string) => {
    const num = parseFloat(value);
    if (!isNaN(num)) {
      const stored = lengthFields.has(field) ? num / factor : num;
      updateWire(wire.id, { [field]: stored } as Partial<Wire>);
    }
  };

  const displayValue = (field: keyof Wire): number => {
    const raw = wire[field] as number;
    return lengthFields.has(field) ? raw * factor : raw;
  };

  const handleTaperChange = (field: 'radiusStart' | 'radiusEnd', value: string) => {
    if (value === '' || value === null) {
      updateWire(wire.id, { [field]: undefined } as Partial<Wire>);
      return;
    }
    const num = parseFloat(value);
    if (!isNaN(num)) {
      updateWire(wire.id, { [field]: num <= 0 ? undefined : num / factor } as Partial<Wire>);
    }
  };

  const rStartDisplay = wire.radiusStart ? parseFloat((wire.radiusStart * factor).toPrecision(6)) : 0;
  const rEndDisplay = wire.radiusEnd ? parseFloat((wire.radiusEnd * factor).toPrecision(6)) : 0;
  const isTapered = (wire.radiusStart ?? 0) > 0 && (wire.radiusEnd ?? 0) > 0;

  // Total column count to span for the advanced row: index + 8 numeric + 4 dropdowns + 3 coating fields + delete
  // = 1 + 8 + 1(material) + 1(coating preset) + 1(coat-t) + 1(εr) + 1(tanδ) + 1(del) = 15
  const totalColumns = 15;

  return (
    <>
    <tr
      className={`wire-row ${isSelected ? 'wire-row-selected' : ''}`}
      onClick={() => selectWire(wire.id)}
    >
      <td>
        <button
          className="btn-small wire-expand-toggle"
          onClick={(e) => { e.stopPropagation(); setExpanded((v) => !v); }}
          title={expanded ? 'Hide advanced' : 'Show advanced (taper)'}
          aria-label="Toggle advanced wire options"
        >
          {expanded ? '▼' : '▶'}
        </button>
        {' '}
        {index + 1}
      </td>
      {numericFields.map((field) => (
        <td key={field}>
          <input
            type="number"
            value={parseFloat(displayValue(field).toPrecision(6))}
            onChange={(e) => handleNumChange(field, e.target.value)}
            step={field === 'radius' ? 0.0001 : field === 'segments' ? 1 : 0.1}
            className={`wire-input ${field === 'radius' && isTapered ? 'wire-input-inactive' : ''}`}
            title={field === 'radius' && isTapered ? 'Tapered wire: edit in advanced row' : undefined}
            disabled={field === 'radius' && isTapered}
          />
        </td>
      ))}
      <td>
        <select
          value={wire.material}
          onChange={(e) => updateWire(wire.id, { material: e.target.value as Material })}
          onClick={(e) => e.stopPropagation()}
          className="wire-input wire-material-select"
          title="Conductor material (skin-effect loss)"
        >
          {materialOptions.map((m) => (
            <option key={m || 'pec'} value={m}>{MATERIAL_LABELS[m]}</option>
          ))}
        </select>
      </td>
      {/* Coating preset selector — fills thickness / εr / tanδ */}
      <td>
        <select
          value={coatingPreset}
          onChange={(e) => applyCoatingPreset(e.target.value)}
          onClick={(e) => e.stopPropagation()}
          className="wire-input wire-material-select"
          title="Apply a standard coating preset"
        >
          {COATING_PRESETS.map((p) => (
            <option key={p.key} value={p.key}>{p.label}</option>
          ))}
        </select>
      </td>
      {/* Coating thickness — unit-converted like radius */}
      <td>
        <input
          type="number"
          value={parseFloat((wire.coatingThickness * factor).toPrecision(6))}
          onChange={(e) => {
            const num = parseFloat(e.target.value);
            if (!isNaN(num)) updateWire(wire.id, { coatingThickness: num / factor });
          }}
          min={0}
          step={0.0001}
          className="wire-input"
          title="Coating shell thickness (0 = bare wire)"
        />
      </td>
      {/* εr — dimensionless */}
      <td>
        <input
          type="number"
          value={wire.coatingEpsR}
          onChange={(e) => {
            const num = parseFloat(e.target.value);
            if (!isNaN(num)) updateWire(wire.id, { coatingEpsR: num });
          }}
          min={1}
          step={0.1}
          className={`wire-input ${wire.coatingThickness <= 0 ? 'wire-input-inactive' : ''}`}
          title="Coating relative permittivity εr (≥1)"
        />
      </td>
      {/* tanδ — dimensionless */}
      <td>
        <input
          type="number"
          value={wire.coatingLossTan}
          onChange={(e) => {
            const num = parseFloat(e.target.value);
            if (!isNaN(num)) updateWire(wire.id, { coatingLossTan: num });
          }}
          min={0}
          step={0.001}
          className={`wire-input ${wire.coatingThickness <= 0 ? 'wire-input-inactive' : ''}`}
          title="Coating loss tangent tanδ (0 = lossless)"
        />
      </td>
      <td>
        <button
          className="btn-small btn-danger"
          onClick={(e) => {
            e.stopPropagation();
            removeWire(wire.id);
          }}
        >
          Del
        </button>
      </td>
    </tr>
    {expanded && (
      <tr
        className={`wire-row wire-row-advanced ${isSelected ? 'wire-row-selected' : ''}`}
        onClick={() => selectWire(wire.id)}
      >
        <td colSpan={totalColumns} className="wire-advanced-cell">
          <div className="wire-advanced-row">
            <label>
              Radius&nbsp;start&nbsp;({UNIT_LABELS[displayUnit]})
              <input
                type="number"
                value={rStartDisplay}
                min={0}
                step={0.0001}
                onChange={(e) => handleTaperChange('radiusStart', e.target.value)}
                onClick={(e) => e.stopPropagation()}
                className="wire-input"
                title="Wire radius at endpoint 1 (x1,y1,z1). 0 = uniform wire."
              />
            </label>
            <label>
              Radius&nbsp;end&nbsp;({UNIT_LABELS[displayUnit]})
              <input
                type="number"
                value={rEndDisplay}
                min={0}
                step={0.0001}
                onChange={(e) => handleTaperChange('radiusEnd', e.target.value)}
                onClick={(e) => e.stopPropagation()}
                className="wire-input"
                title="Wire radius at endpoint 2 (x2,y2,z2). 0 = uniform wire."
              />
            </label>
            <span className="wire-advanced-hint">
              Set both to enable a linear taper; leave either blank (or 0) for a uniform wire.
            </span>
          </div>
        </td>
      </tr>
    )}
    </>
  );
};

export default WireRow;
