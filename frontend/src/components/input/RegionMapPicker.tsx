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
 * Interactive world-map picker for ground εr / σ.
 *
 * Modal overlay with an equirectangular-projection SVG world map. In Navigate
 * mode the user clicks a region to preview its soil values and double-clicks
 * to apply them. In Draw mode the user traces a polygon vertex-by-vertex and
 * saves it with a name + εr / σ to localStorage, overriding the base ITU-R
 * P.832 zones underneath. Smallest-by-area wins when polygons overlap, so
 * user polygons naturally refine the base zones.
 *
 * The modal is fixed-position; the parent decides when it's open via the
 * `open` prop. Applying or closing both dismiss the overlay.
 */
import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  ITU_P832_ZONES,
  type GroundRegion,
  type ItuZoneDef,
} from '@/types';
import {
  projectEquirect,
  unprojectEquirect,
  smallestContainingRegion,
  loadItuRegions,
} from '@/utils/geo';
import {
  loadUserRegions,
  addUserRegion,
  deleteUserRegion,
  updateUserRegion,
  exportUserRegions,
  parseUserRegionImport,
  importUserRegions,
} from '@/utils/regionStorage';
import ituData from '@/data/itu_r_p832.json';

const MAP_W = 960;
const MAP_H = 480;
const MIN_ZOOM = 1;
const MAX_ZOOM = 16;

interface Props {
  open: boolean;
  onClose: () => void;
  onApply: (values: { epsR: number; sigma: number; regionPreset: string }) => void;
}

interface PendingPolygon {
  vertices: [number, number][]; // lon, lat
  cursor?: [number, number];    // live cursor for rubber-band segment
}

interface NewRegionDraft {
  name: string;
  epsR: number;
  sigma: number;
  polygon: [number, number][];
}

function newId(): string {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) {
    return crypto.randomUUID();
  }
  return `user-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

const RegionMapPicker: React.FC<Props> = ({ open, onClose, onApply }) => {
  const zoneLookup = useMemo(() => {
    const m: Record<number, ItuZoneDef> = {};
    for (const z of ITU_P832_ZONES) m[z.zone] = z;
    return m;
  }, []);

  const ituRegions = useMemo(
    () => loadItuRegions(ituData as Parameters<typeof loadItuRegions>[0], zoneLookup),
    [zoneLookup],
  );

  const [userRegions, setUserRegions] = useState<GroundRegion[]>(() => loadUserRegions());
  const allRegions = useMemo(() => [...ituRegions, ...userRegions], [ituRegions, userRegions]);

  const [mode, setMode] = useState<'navigate' | 'draw'>('navigate');
  const [zoom, setZoom] = useState(1);
  const [pan, setPan] = useState({ x: 0, y: 0 });
  const [hovered, setHovered] = useState<GroundRegion | null>(null);
  const [selected, setSelected] = useState<GroundRegion | null>(null);
  const [pending, setPending] = useState<PendingPolygon | null>(null);
  const [draft, setDraft] = useState<NewRegionDraft | null>(null);
  const [importError, setImportError] = useState<string | null>(null);
  const [importStatus, setImportStatus] = useState<string | null>(null);

  const svgRef = useRef<SVGSVGElement | null>(null);
  const panDragRef = useRef<{ x: number; y: number; startPan: { x: number; y: number } } | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  // Reset transient state when modal is closed/reopened.
  useEffect(() => {
    if (!open) return;
    setMode('navigate');
    setZoom(1);
    setPan({ x: 0, y: 0 });
    setHovered(null);
    setSelected(null);
    setPending(null);
    setDraft(null);
    setImportError(null);
    setImportStatus(null);
    setUserRegions(loadUserRegions());
  }, [open]);

  // Keyboard shortcuts: Escape dismisses the modal (or cancels a draw in progress).
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        if (draft) {
          setDraft(null);
        } else if (pending) {
          setPending(null);
        } else {
          onClose();
        }
      } else if (e.key === 'Enter' && pending && pending.vertices.length >= 3) {
        finalisePolygon();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [open, pending, draft, onClose]);

  if (!open) return null;

  // --- Coordinate helpers --------------------------------------------------

  // SVG pointer → viewBox pixel coords (accounts for the DOM-to-SVG scaling).
  function pointerToSvg(e: React.PointerEvent<SVGSVGElement>): [number, number] {
    const svg = e.currentTarget;
    const rect = svg.getBoundingClientRect();
    const px = ((e.clientX - rect.left) / rect.width) * MAP_W;
    const py = ((e.clientY - rect.top) / rect.height) * MAP_H;
    return [px, py];
  }

  // SVG pixel → map pixel (inverse of the zoom/pan transform on the inner <g>).
  function svgToMap(px: number, py: number): [number, number] {
    const mx = (px - pan.x) / zoom;
    const my = (py - pan.y) / zoom;
    return [mx, my];
  }

  // Map pixel → (lon, lat).
  function mapToLonLat(mx: number, my: number): [number, number] {
    return unprojectEquirect(mx, my, MAP_W, MAP_H);
  }

  function pointerToLonLat(e: React.PointerEvent<SVGSVGElement>): [number, number] {
    const [sx, sy] = pointerToSvg(e);
    const [mx, my] = svgToMap(sx, sy);
    return mapToLonLat(mx, my);
  }

  function polygonPath(ring: [number, number][]): string {
    if (ring.length === 0) return '';
    const parts = ring.map(([lon, lat], i) => {
      const [x, y] = projectEquirect(lon, lat, MAP_W, MAP_H);
      return `${i === 0 ? 'M' : 'L'}${x.toFixed(2)} ${y.toFixed(2)}`;
    });
    return parts.join(' ') + ' Z';
  }

  // --- Interaction handlers ------------------------------------------------

  const onMapPointerDown = (e: React.PointerEvent<SVGSVGElement>) => {
    if (e.button !== 0) return;
    if (mode === 'draw') {
      // Click in draw mode appends a vertex; no panning.
      const [lon, lat] = pointerToLonLat(e);
      setPending((prev) =>
        prev
          ? { ...prev, vertices: [...prev.vertices, [lon, lat]] }
          : { vertices: [[lon, lat]] },
      );
      return;
    }
    // Navigate mode: start a pan drag.
    e.currentTarget.setPointerCapture?.(e.pointerId);
    panDragRef.current = { x: e.clientX, y: e.clientY, startPan: { ...pan } };
  };

  const onMapPointerMove = (e: React.PointerEvent<SVGSVGElement>) => {
    if (mode === 'draw' && pending) {
      const [lon, lat] = pointerToLonLat(e);
      setPending({ ...pending, cursor: [lon, lat] });
      return;
    }
    if (panDragRef.current) {
      const rect = e.currentTarget.getBoundingClientRect();
      const scaleX = MAP_W / rect.width;
      const scaleY = MAP_H / rect.height;
      const dx = (e.clientX - panDragRef.current.x) * scaleX;
      const dy = (e.clientY - panDragRef.current.y) * scaleY;
      setPan({
        x: panDragRef.current.startPan.x + dx,
        y: panDragRef.current.startPan.y + dy,
      });
      return;
    }
    // Hover hit-test in navigate mode.
    if (mode === 'navigate') {
      const [lon, lat] = pointerToLonLat(e);
      setHovered(smallestContainingRegion(allRegions, lon, lat));
    }
  };

  const onMapPointerUp = (e: React.PointerEvent<SVGSVGElement>) => {
    panDragRef.current = null;
    if (mode === 'navigate') {
      const [lon, lat] = pointerToLonLat(e);
      setSelected(smallestContainingRegion(allRegions, lon, lat));
    }
  };

  const onMapDoubleClick = (e: React.PointerEvent<SVGSVGElement>) => {
    if (mode === 'draw' && pending && pending.vertices.length >= 3) {
      e.preventDefault();
      finalisePolygon();
      return;
    }
    if (mode === 'navigate') {
      const [lon, lat] = pointerToLonLat(e);
      const hit = smallestContainingRegion(allRegions, lon, lat);
      if (hit) {
        applyRegion(hit);
      }
    }
  };

  const onWheel = (e: React.WheelEvent<SVGSVGElement>) => {
    e.preventDefault();
    const [sx, sy] = [
      ((e.clientX - e.currentTarget.getBoundingClientRect().left) / e.currentTarget.getBoundingClientRect().width) * MAP_W,
      ((e.clientY - e.currentTarget.getBoundingClientRect().top) / e.currentTarget.getBoundingClientRect().height) * MAP_H,
    ];
    const factor = e.deltaY < 0 ? 1.2 : 1 / 1.2;
    const newZoom = Math.max(MIN_ZOOM, Math.min(MAX_ZOOM, zoom * factor));
    // Zoom towards the cursor: preserve the point (sx, sy) in map space.
    const [mxBefore, myBefore] = svgToMap(sx, sy);
    const newPanX = sx - mxBefore * newZoom;
    const newPanY = sy - myBefore * newZoom;
    setZoom(newZoom);
    setPan({ x: newPanX, y: newPanY });
  };

  // --- Draw mode finalisation --------------------------------------------

  function finalisePolygon() {
    if (!pending || pending.vertices.length < 3) return;
    // Default the draft fields from the region currently under the polygon
    // centroid so the form is pre-populated with something sensible.
    const centroid = polygonCentroid(pending.vertices);
    const underlying = smallestContainingRegion(allRegions, centroid[0], centroid[1]);
    setDraft({
      name: `My region ${userRegions.length + 1}`,
      epsR: underlying?.epsR ?? 13,
      sigma: underlying?.sigma ?? 0.005,
      polygon: pending.vertices,
    });
    setPending(null);
  }

  function saveDraft() {
    if (!draft) return;
    if (!(draft.epsR >= 1) || !(draft.sigma > 0)) return;
    const region: GroundRegion = {
      id: newId(),
      name: draft.name || 'Untitled region',
      source: 'user',
      polygon: draft.polygon,
      epsR: draft.epsR,
      sigma: draft.sigma,
    };
    const updated = addUserRegion(region);
    setUserRegions(updated);
    setDraft(null);
    setMode('navigate');
  }

  function applyRegion(r: GroundRegion) {
    const label = r.source === 'itu'
      ? `itu:${r.zone ?? 0}`
      : `user:${r.id}`;
    onApply({ epsR: r.epsR, sigma: r.sigma, regionPreset: label });
    onClose();
  }

  function handleExport() {
    if (userRegions.length === 0) return;
    const text = exportUserRegions(userRegions);
    const blob = new Blob([text], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    const today = new Date().toISOString().slice(0, 10);
    a.href = url;
    a.download = `ve3ksm-regions-${today}.json`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  }

  function handleImportClick() {
    setImportError(null);
    setImportStatus(null);
    fileInputRef.current?.click();
  }

  function handleImportChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    // Clear the input so the same file can be re-selected later.
    if (e.target) e.target.value = '';
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => {
      const text = typeof reader.result === 'string' ? reader.result : '';
      const result = parseUserRegionImport(text);
      if (!result.ok) {
        setImportError(result.error);
        setImportStatus(null);
        return;
      }
      if (result.regions.length === 0) {
        setImportError(`No valid regions found${result.skipped ? ` (${result.skipped} skipped)` : ''}`);
        return;
      }
      const merged = importUserRegions(result.regions, newId);
      setUserRegions(merged);
      setImportError(null);
      const skippedNote = result.skipped > 0 ? ` · ${result.skipped} skipped` : '';
      setImportStatus(`Imported ${result.regions.length}${skippedNote} — ${merged.length} total`);
      // Clear status after a few seconds to return to the normal count.
      setTimeout(() => setImportStatus(null), 3500);
    };
    reader.onerror = () => {
      setImportError('Could not read the file');
    };
    reader.readAsText(file);
  }

  function handleDeleteUser(id: string) {
    setUserRegions(deleteUserRegion(id));
    if (selected && selected.id === id) setSelected(null);
    if (hovered && hovered.id === id) setHovered(null);
  }

  function handleEditUser(id: string, patch: Partial<GroundRegion>) {
    setUserRegions(updateUserRegion(id, patch));
  }

  // --- Render -------------------------------------------------------------

  const pendingPath = pending
    ? polygonPath(
        pending.cursor ? [...pending.vertices, pending.cursor] : pending.vertices,
      )
    : '';

  return (
    <div className="region-modal-overlay" onClick={onClose}>
      <div className="region-modal" onClick={(e) => e.stopPropagation()}>
        <header className="region-modal-header">
          <h3>Region Picker</h3>
          <div className="region-mode-tabs">
            <button
              className={mode === 'navigate' ? 'tab-active' : ''}
              onClick={() => { setMode('navigate'); setPending(null); }}
            >Navigate</button>
            <button
              className={mode === 'draw' ? 'tab-active' : ''}
              onClick={() => { setMode('draw'); setSelected(null); setHovered(null); }}
            >Draw polygon</button>
          </div>
          <div className="region-zoom-controls">
            <button onClick={() => setZoom((z) => Math.max(MIN_ZOOM, z / 1.5))}>−</button>
            <span>{(zoom * 100).toFixed(0)}%</span>
            <button onClick={() => setZoom((z) => Math.min(MAX_ZOOM, z * 1.5))}>+</button>
            <button onClick={() => { setZoom(1); setPan({ x: 0, y: 0 }); }}>Reset</button>
          </div>
          <button className="region-modal-close" onClick={onClose} title="Close">✕</button>
        </header>

        <div className="region-modal-body">
          <div className="region-map-wrapper">
            <svg
              ref={svgRef}
              className={`region-map ${mode === 'draw' ? 'drawing' : ''}`}
              viewBox={`0 0 ${MAP_W} ${MAP_H}`}
              preserveAspectRatio="xMidYMid meet"
              onPointerDown={onMapPointerDown}
              onPointerMove={onMapPointerMove}
              onPointerUp={onMapPointerUp}
              onPointerLeave={() => { setHovered(null); panDragRef.current = null; }}
              onDoubleClick={onMapDoubleClick}
              onWheel={onWheel}
            >
              <rect x={0} y={0} width={MAP_W} height={MAP_H} className="region-ocean-bg" />
              <g transform={`translate(${pan.x} ${pan.y}) scale(${zoom})`}>
                {/* Render base zones first, then user regions on top. */}
                {ituRegions.map((r) => (
                  <path
                    key={r.id}
                    d={polygonPath(r.polygon)}
                    fill={zoneLookup[r.zone ?? 0]?.colour ?? '#555'}
                    stroke={hovered?.id === r.id ? 'var(--accent)' : 'rgba(0,0,0,0.35)'}
                    strokeWidth={hovered?.id === r.id ? 2 / zoom : 0.5 / zoom}
                    opacity={0.85}
                  />
                ))}
                {userRegions.map((r) => (
                  <path
                    key={r.id}
                    d={polygonPath(r.polygon)}
                    fill="var(--accent-secondary)"
                    stroke={hovered?.id === r.id || selected?.id === r.id
                      ? 'var(--accent)' : 'rgba(0,0,0,0.5)'}
                    strokeWidth={1.5 / zoom}
                    opacity={0.75}
                  />
                ))}
                {/* Graticule: 30° lat/lon lines. */}
                {[-60, -30, 0, 30, 60].map((lat) => {
                  const [, y] = projectEquirect(0, lat, MAP_W, MAP_H);
                  return (
                    <line key={`lat${lat}`}
                      x1={0} y1={y} x2={MAP_W} y2={y}
                      stroke="rgba(255,255,255,0.08)" strokeWidth={0.5 / zoom} />
                  );
                })}
                {[-120, -60, 0, 60, 120].map((lon) => {
                  const [x] = projectEquirect(lon, 0, MAP_W, MAP_H);
                  return (
                    <line key={`lon${lon}`}
                      x1={x} y1={0} x2={x} y2={MAP_H}
                      stroke="rgba(255,255,255,0.08)" strokeWidth={0.5 / zoom} />
                  );
                })}
                {/* In-progress polygon. */}
                {pending && (
                  <path
                    d={pendingPath}
                    fill="rgba(68,136,255,0.2)"
                    stroke="var(--accent)"
                    strokeWidth={1.5 / zoom}
                    strokeDasharray={`${4 / zoom} ${3 / zoom}`}
                  />
                )}
                {pending && pending.vertices.map(([lon, lat], i) => {
                  const [x, y] = projectEquirect(lon, lat, MAP_W, MAP_H);
                  return (
                    <circle key={i} cx={x} cy={y} r={3 / zoom}
                      fill="var(--accent)" stroke="white" strokeWidth={0.5 / zoom} />
                  );
                })}
              </g>
            </svg>
            <div className="region-map-hint">
              {mode === 'navigate'
                ? 'Click to select · double-click to apply · scroll to zoom · drag to pan'
                : `Click to add vertex · double-click (or Enter) to close · Escape to cancel · ${pending?.vertices.length ?? 0} vertices`}
            </div>
          </div>

          <aside className="region-sidebar">
            <section>
              <h4>{selected ? selected.name : hovered ? hovered.name : 'No region'}</h4>
              {(selected ?? hovered) && (
                <>
                  <p className="muted small">
                    {(selected ?? hovered)!.source === 'itu'
                      ? `ITU-R P.832 zone ${(selected ?? hovered)!.zone}`
                      : 'User-drawn'}
                  </p>
                  <div className="region-values">
                    <span>εr = {(selected ?? hovered)!.epsR}</span>
                    <span>σ = {(selected ?? hovered)!.sigma} S/m</span>
                  </div>
                </>
              )}
              <button
                className="region-apply-btn"
                disabled={!selected}
                onClick={() => selected && applyRegion(selected)}
              >
                Apply to ground config
              </button>
            </section>

            <section>
              <h4>Base zones</h4>
              <ul className="region-zone-legend">
                {ITU_P832_ZONES.map((z) => (
                  <li key={z.zone}>
                    <span className="swatch" style={{ background: z.colour }} />
                    {z.label} — εr={z.epsR}, σ={z.sigma} S/m
                  </li>
                ))}
              </ul>
            </section>

            <section>
              <h4>
                {importStatus
                  ? importStatus
                  : `My regions (${userRegions.length})`}
              </h4>
              <div className="region-io-row">
                <button
                  onClick={handleExport}
                  disabled={userRegions.length === 0}
                  title={userRegions.length === 0
                    ? 'Draw a polygon first to enable export'
                    : 'Download all user regions as JSON'}
                >↓ Export</button>
                <button
                  onClick={handleImportClick}
                  title="Merge regions from a previously-exported JSON file"
                >↑ Import</button>
                <input
                  ref={fileInputRef}
                  type="file"
                  accept="application/json,.json"
                  style={{ display: 'none' }}
                  onChange={handleImportChange}
                />
              </div>
              {importError && (
                <p className="region-import-error">{importError}</p>
              )}
              {userRegions.length === 0
                ? <p className="muted small">Switch to Draw polygon mode to add refinements.</p>
                : (
                  <ul className="region-user-list">
                    {userRegions.map((r) => (
                      <li key={r.id}>
                        <div className="region-user-row">
                          <input
                            type="text"
                            value={r.name}
                            onChange={(e) => handleEditUser(r.id, { name: e.target.value })}
                          />
                          <button onClick={() => applyRegion(r)} title="Apply">↑</button>
                          <button onClick={() => handleDeleteUser(r.id)} title="Delete">✕</button>
                        </div>
                        <div className="region-user-values">
                          <label>εr
                            <input type="number" value={r.epsR} min={1} step={0.1}
                              onChange={(e) => {
                                const n = parseFloat(e.target.value);
                                if (!Number.isNaN(n)) handleEditUser(r.id, { epsR: n });
                              }} />
                          </label>
                          <label>σ
                            <input type="number" value={r.sigma} min={0} step={0.001}
                              onChange={(e) => {
                                const n = parseFloat(e.target.value);
                                if (!Number.isNaN(n)) handleEditUser(r.id, { sigma: n });
                              }} />
                          </label>
                        </div>
                      </li>
                    ))}
                  </ul>
                )}
            </section>
          </aside>
        </div>

        {/* New-region form appears after closing a drawn polygon. */}
        {draft && (
          <div className="region-draft-form">
            <strong>Save new region</strong>
            <label>Name
              <input type="text" value={draft.name}
                onChange={(e) => setDraft({ ...draft, name: e.target.value })} />
            </label>
            <label>εr
              <input type="number" min={1} step={0.1} value={draft.epsR}
                onChange={(e) => {
                  const n = parseFloat(e.target.value);
                  if (!Number.isNaN(n)) setDraft({ ...draft, epsR: n });
                }} />
            </label>
            <label>σ (S/m)
              <input type="number" min={0} step={0.001} value={draft.sigma}
                onChange={(e) => {
                  const n = parseFloat(e.target.value);
                  if (!Number.isNaN(n)) setDraft({ ...draft, sigma: n });
                }} />
            </label>
            <button onClick={saveDraft}>Save</button>
            <button onClick={() => setDraft(null)}>Cancel</button>
          </div>
        )}
      </div>
    </div>
  );
};

/** Area-weighted centroid of a non-self-intersecting polygon (shoelace). */
function polygonCentroid(ring: [number, number][]): [number, number] {
  let cx = 0, cy = 0, a = 0;
  const n = ring.length;
  for (let i = 0, j = n - 1; i < n; j = i++) {
    const [xi, yi] = ring[i];
    const [xj, yj] = ring[j];
    const f = xj * yi - xi * yj;
    cx += (xj + xi) * f;
    cy += (yj + yi) * f;
    a += f;
  }
  a *= 0.5;
  if (Math.abs(a) < 1e-9) return ring[0];
  return [cx / (6 * a), cy / (6 * a)];
}

export default RegionMapPicker;
