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
 * localStorage persistence for user-drawn ground regions.
 *
 * User polygons live per-browser; they do NOT travel with exported antenna
 * design files. A schema-version guard lets us evolve the stored shape
 * without crashing older saved data.
 */
import type { GroundRegion } from '@/types';

const STORAGE_KEY = 've3ksm.userRegions';
const SCHEMA_VERSION = 1;

interface StoredPayload {
  version: number;
  regions: GroundRegion[];
}

/** Read all user regions from localStorage. Returns an empty array on any
 *  parse failure, missing key, or schema-version mismatch — we never throw. */
export function loadUserRegions(): GroundRegion[] {
  if (typeof window === 'undefined' || !window.localStorage) return [];
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as StoredPayload;
    if (!parsed || parsed.version !== SCHEMA_VERSION || !Array.isArray(parsed.regions)) {
      return [];
    }
    return parsed.regions.filter((r) => r.source === 'user');
  } catch {
    return [];
  }
}

/** Overwrite the full user-region list in localStorage. */
export function saveUserRegions(regions: GroundRegion[]): void {
  if (typeof window === 'undefined' || !window.localStorage) return;
  const payload: StoredPayload = { version: SCHEMA_VERSION, regions };
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(payload));
  } catch {
    // Quota exceeded or storage disabled — silently give up; the user can
    // re-draw if needed. We don't want a failed save to crash the modal.
  }
}

/** Append a new user region. Returns the full updated list. */
export function addUserRegion(region: GroundRegion): GroundRegion[] {
  const list = loadUserRegions();
  list.push(region);
  saveUserRegions(list);
  return list;
}

/** Update an existing user region by id (shallow merge). Returns the full list. */
export function updateUserRegion(
  id: string,
  patch: Partial<GroundRegion>,
): GroundRegion[] {
  const list = loadUserRegions().map((r) => (r.id === id ? { ...r, ...patch } : r));
  saveUserRegions(list);
  return list;
}

/** Remove a user region by id. Returns the full updated list. */
export function deleteUserRegion(id: string): GroundRegion[] {
  const list = loadUserRegions().filter((r) => r.id !== id);
  saveUserRegions(list);
  return list;
}

// --- JSON export / import ----------------------------------------------------

const EXPORT_KIND = 'user-regions';
const EXPORT_APP = 've3ksm-antenna-studio';

interface ExportFile {
  app: string;
  kind: string;
  version: number;
  exportedAt: string;
  regions: GroundRegion[];
}

/** Serialise the given user regions to a pretty-printed JSON string with the
 *  wrapper schema (app, kind, version, exportedAt). The returned text is ready
 *  for a Blob download. */
export function exportUserRegions(regions: GroundRegion[]): string {
  const payload: ExportFile = {
    app: EXPORT_APP,
    kind: EXPORT_KIND,
    version: SCHEMA_VERSION,
    exportedAt: new Date().toISOString(),
    regions: regions.filter((r) => r.source === 'user'),
  };
  return JSON.stringify(payload, null, 2);
}

/** Result of a parse attempt: either a list of valid regions plus a count of
 *  skipped bad features, or an error string. */
export type ImportParseResult =
  | { ok: true; regions: GroundRegion[]; skipped: number }
  | { ok: false; error: string };

function isValidRing(poly: unknown): poly is [number, number][] {
  if (!Array.isArray(poly) || poly.length < 3) return false;
  for (const v of poly) {
    if (!Array.isArray(v) || v.length !== 2) return false;
    const [lon, lat] = v as unknown[];
    if (typeof lon !== 'number' || typeof lat !== 'number') return false;
    if (!Number.isFinite(lon) || !Number.isFinite(lat)) return false;
  }
  return true;
}

/** Parse and validate an imported JSON file. Never throws; returns a tagged
 *  result so callers can branch on success. Skips individual malformed
 *  features but aborts on schema mismatches (wrong app/kind/version). */
export function parseUserRegionImport(text: string): ImportParseResult {
  let obj: unknown;
  try {
    obj = JSON.parse(text);
  } catch (err) {
    return { ok: false, error: `Invalid JSON: ${(err as Error).message}` };
  }
  if (!obj || typeof obj !== 'object') {
    return { ok: false, error: 'File is not a JSON object' };
  }
  const p = obj as Partial<ExportFile>;
  if (p.kind !== EXPORT_KIND) {
    return { ok: false, error: `Wrong file kind: expected "${EXPORT_KIND}", got "${p.kind ?? '(missing)'}"` };
  }
  if (p.version !== SCHEMA_VERSION) {
    return { ok: false, error: `Unsupported schema version ${p.version ?? '(missing)'} (expected ${SCHEMA_VERSION})` };
  }
  if (!Array.isArray(p.regions)) {
    return { ok: false, error: 'Missing "regions" array' };
  }

  const valid: GroundRegion[] = [];
  let skipped = 0;
  for (const raw of p.regions) {
    if (!raw || typeof raw !== 'object') { skipped++; continue; }
    const r = raw as Partial<GroundRegion>;
    if (typeof r.name !== 'string' || !r.name.trim()) { skipped++; continue; }
    if (typeof r.epsR !== 'number' || !(r.epsR >= 1)) { skipped++; continue; }
    if (typeof r.sigma !== 'number' || !(r.sigma > 0)) { skipped++; continue; }
    if (!isValidRing(r.polygon)) { skipped++; continue; }
    valid.push({
      id: typeof r.id === 'string' ? r.id : '',
      name: r.name,
      source: 'user',
      polygon: r.polygon,
      epsR: r.epsR,
      sigma: r.sigma,
    });
  }
  return { ok: true, regions: valid, skipped };
}

/** Append a freshly-parsed batch to existing localStorage regions, always
 *  regenerating IDs so double-imports produce distinct copies. Returns the
 *  full merged list. */
export function importUserRegions(
  incoming: GroundRegion[],
  makeId: () => string,
): GroundRegion[] {
  const existing = loadUserRegions();
  const merged = [
    ...existing,
    ...incoming.map((r) => ({ ...r, id: makeId(), source: 'user' as const })),
  ];
  saveUserRegions(merged);
  return merged;
}
