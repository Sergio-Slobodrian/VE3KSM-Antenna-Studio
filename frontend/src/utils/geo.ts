/**
 * Geography helpers for the RegionMapPicker.
 *
 * All functions are pure and use WGS84 degrees as input. The projection is
 * equirectangular — "flattened" per the feature spec — so that drawing to SVG
 * reduces to a linear rescale with no trig. Point-in-polygon uses the standard
 * ray-casting algorithm; area uses the shoelace formula in unprojected
 * degree² space (sufficient for the smallest-polygon-wins priority rule, which
 * only needs a monotone ordering).
 */
import type { GroundRegion } from '@/types';

/** Equirectangular projection into a width x height pixel box.
 *  lon ∈ [-180, 180] maps to x ∈ [0, width]; lat ∈ [-90, 90] maps to y ∈ [0, height]
 *  with +lat = up (SVG y-axis is flipped, so north is up visually). */
export function projectEquirect(
  lon: number,
  lat: number,
  width: number,
  height: number,
): [number, number] {
  const x = ((lon + 180) / 360) * width;
  const y = ((90 - lat) / 180) * height;
  return [x, y];
}

/** Inverse projection: SVG (x, y) → (lon, lat). */
export function unprojectEquirect(
  x: number,
  y: number,
  width: number,
  height: number,
): [number, number] {
  const lon = (x / width) * 360 - 180;
  const lat = 90 - (y / height) * 180;
  return [lon, lat];
}

/** Standard ray-casting point-in-polygon test.
 *  Returns true when the (lon, lat) point is inside the closed ring. */
export function pointInPolygon(
  lon: number,
  lat: number,
  ring: [number, number][],
): boolean {
  let inside = false;
  const n = ring.length;
  for (let i = 0, j = n - 1; i < n; j = i++) {
    const [xi, yi] = ring[i];
    const [xj, yj] = ring[j];
    const intersect =
      yi > lat !== yj > lat &&
      lon < ((xj - xi) * (lat - yi)) / (yj - yi || Number.EPSILON) + xi;
    if (intersect) inside = !inside;
  }
  return inside;
}

/** Absolute area of a polygon ring in degree² (shoelace formula).
 *  Not geographically accurate — used only to order regions by size so the
 *  smallest containing polygon wins the lookup. */
export function ringAreaDeg2(ring: [number, number][]): number {
  let sum = 0;
  const n = ring.length;
  for (let i = 0, j = n - 1; i < n; j = i++) {
    const [xi, yi] = ring[i];
    const [xj, yj] = ring[j];
    sum += (xj + xi) * (yj - yi);
  }
  return Math.abs(sum) / 2;
}

/** Pick the smallest-by-area region that contains (lon, lat). Returns null
 *  when no region covers the point. User polygons naturally beat base zones
 *  because they are smaller; the explicit area sort guarantees the priority
 *  even for pathological cases. */
export function smallestContainingRegion(
  regions: GroundRegion[],
  lon: number,
  lat: number,
): GroundRegion | null {
  let best: GroundRegion | null = null;
  let bestArea = Infinity;
  for (const r of regions) {
    if (!pointInPolygon(lon, lat, r.polygon)) continue;
    const a = ringAreaDeg2(r.polygon);
    if (a < bestArea) {
      best = r;
      bestArea = a;
    }
  }
  return best;
}

/** Load the bundled ITU-R P.832 placeholder data into the picker's internal
 *  GroundRegion format. Keeps the data file unopinionated (proper GeoJSON) so
 *  it can be regenerated from Natural Earth / ITU atlas tooling later. */
export function loadItuRegions(
  geojson: {
    features: {
      properties: { id: string; name: string; zone: number };
      geometry: { coordinates: number[][][] };
    }[];
  },
  zoneLookup: Record<number, { epsR: number; sigma: number }>,
): GroundRegion[] {
  const out: GroundRegion[] = [];
  for (const f of geojson.features) {
    const z = zoneLookup[f.properties.zone];
    if (!z) continue;
    // GeoJSON polygons are nested one level deep (outer ring + optional holes);
    // the bootstrap data has no holes so we only consume the outer ring.
    const ring = f.geometry.coordinates[0] as [number, number][];
    out.push({
      id: f.properties.id,
      name: f.properties.name,
      source: 'itu',
      zone: f.properties.zone,
      polygon: ring,
      epsR: z.epsR,
      sigma: z.sigma,
    });
  }
  return out;
}
