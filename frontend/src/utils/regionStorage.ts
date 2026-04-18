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
