/**
 * 3D radiation pattern viewer using Three.js.
 *
 * Builds a triangulated surface mesh from the simulation's far-field pattern
 * data (theta/phi grid of gain values).  The radial distance of each vertex
 * is derived from the dB gain via a log-power mapping, so lobes are visually
 * proportional to gain.  Vertex colours use an HSL ramp from blue (low gain)
 * through green/yellow to red (high gain).
 *
 * Coordinate mapping: physics spherical (theta from +Z, phi in XY) is
 * converted to Three.js Y-up Cartesian via `sphericalToCartesian`.
 */
import React, { useMemo } from 'react';
import { Canvas } from '@react-three/fiber';
import { OrbitControls } from '@react-three/drei';
import * as THREE from 'three';
import { useAntennaStore } from '@/store/antennaStore';
import { sphericalToCartesian } from '@/utils/conversions';
import ColorScale from '@/components/common/ColorScale';

/** Builds and renders the 3D radiation-pattern surface mesh. */
const PatternMesh: React.FC = () => {
  const simulationResult = useAntennaStore((s) => s.simulationResult);
  const groundType = useAntennaStore((s) => s.ground.type);

  const geometry = useMemo(() => {
    if (!simulationResult || simulationResult.pattern.length === 0) return null;

    const pattern = simulationResult.pattern;
    // Filter out suppressed points (e.g. below ground at -100 dB) when computing
    // the gain range, so they don't inflate the range and distort the radius mapping.
    const SUPPRESSED_DB = -99;
    const hasGround = groundType !== 'free_space';
    const activeGains = pattern
      .filter((p) => p.gainDb > SUPPRESSED_DB && !(hasGround && p.theta > 90))
      .map((p) => p.gainDb);
    const minGain = activeGains.length > 0 ? Math.min(...activeGains) : -10;
    const maxGain = activeGains.length > 0 ? Math.max(...activeGains) : 0;
    const gainRange = maxGain - minGain || 1;

    // Extract sorted unique theta and phi values to form the grid axes
    const thetaSet = new Set<number>();
    const phiSet = new Set<number>();
    pattern.forEach((p) => {
      thetaSet.add(p.theta);
      phiSet.add(p.phi);
    });

    const thetas = Array.from(thetaSet).sort((a, b) => a - b);
    const phis = Array.from(phiSet).sort((a, b) => a - b);

    if (thetas.length < 2 || phis.length < 2) return null;

    // Fast lookup: "theta,phi" -> gainDb for grid vertex generation
    const gainMap = new Map<string, number>();
    pattern.forEach((p) => {
      gainMap.set(`${p.theta},${p.phi}`, p.gainDb);
    });

    const positions: number[] = [];
    const colors: number[] = [];
    const indices: number[] = [];

    // Track which vertices are suppressed (below ground) so we can skip
    // triangles that would stretch from real lobes down to the origin.
    const suppressed: boolean[] = [];

    // Generate vertices: radial distance from dB gain, colour from normalised gain
    for (let ti = 0; ti < thetas.length; ti++) {
      for (let pi = 0; pi < phis.length; pi++) {
        const theta = thetas[ti];
        const phi = phis[pi];
        const gain = gainMap.get(`${theta},${phi}`) ?? minGain;

        // Suppress below-ground points: either backend flagged them as -100 dB,
        // or a ground plane is configured and theta > 90° (below horizon).
        // For "real" ground the Sommerfeld model is not yet implemented, so we
        // suppress the lower hemisphere to avoid showing a misleading mirror image.
        const isBelowGround = gain <= SUPPRESSED_DB ||
          (groundType !== 'free_space' && theta > 90);

        if (isBelowGround) {
          positions.push(0, 0, 0);
          colors.push(0, 0, 0);
          suppressed.push(true);
        } else {
          const normalized = (gain - minGain) / gainRange;
          // Log-power radius: offset +3 dB so the weakest active lobe is still visible
          const r = Math.pow(10, (gain - minGain + 3) / 20) * 0.5;

          const { x, y, z } = sphericalToCartesian(r, theta, phi);
          positions.push(x, y, z);

          // HSL colour ramp: hue 0.66 (blue) at low gain down to 0 (red) at high gain
          const color = new THREE.Color();
          color.setHSL((1 - normalized) * 0.66, 1.0, 0.5);
          colors.push(color.r, color.g, color.b);
          suppressed.push(false);
        }
      }
    }

    // Generate triangle indices, skipping any quad where a corner is suppressed.
    // This prevents triangles stretching from above-ground lobes to the origin.
    for (let ti = 0; ti < thetas.length - 1; ti++) {
      for (let pi = 0; pi < phis.length - 1; pi++) {
        const a = ti * phis.length + pi;
        const b = a + 1;
        const c = (ti + 1) * phis.length + pi;
        const d = c + 1;

        if (suppressed[a] || suppressed[b] || suppressed[c] || suppressed[d]) {
          continue;
        }

        indices.push(a, b, c);
        indices.push(b, d, c);
      }
    }

    const geom = new THREE.BufferGeometry();
    geom.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
    geom.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
    geom.setIndex(indices);
    geom.computeVertexNormals();

    return geom;
  }, [simulationResult]);

  if (!geometry) return null;

  return (
    <mesh geometry={geometry}>
      <meshStandardMaterial vertexColors side={THREE.DoubleSide} transparent opacity={0.6} depthWrite={false} />
    </mesh>
  );
};

/** Outer component: shows the 3D pattern canvas with a colour-scale legend overlay. */
const PatternViewer: React.FC = () => {
  const simulationResult = useAntennaStore((s) => s.simulationResult);

  if (!simulationResult || simulationResult.pattern.length === 0) {
    return (
      <div className="no-data-message">
        <p>No radiation pattern data.</p>
        <p>Run a simulation to see the 3D radiation pattern.</p>
      </div>
    );
  }

  const activeGains = simulationResult.pattern.map((p) => p.gainDb).filter((g) => g > -99);
  const minGain = activeGains.length > 0 ? Math.min(...activeGains) : -10;
  const maxGain = activeGains.length > 0 ? Math.max(...activeGains) : 0;

  return (
    <div className="editor-container" style={{ position: 'relative' }}>
      <Canvas camera={{ position: [5, 5, 5], fov: 50, near: 0.01, far: 10000 }}>
        <ambientLight intensity={0.4} />
        <directionalLight position={[5, 10, 5]} intensity={0.7} />
        <PatternMesh />
        {useAntennaStore.getState().ground.type !== 'free_space' && (
          <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, 0, 0]}>
            <planeGeometry args={[12, 12]} />
            <meshStandardMaterial color="#556655" transparent opacity={0.2} side={THREE.DoubleSide} />
          </mesh>
        )}
        <axesHelper args={[3]} />
        <OrbitControls makeDefault />
      </Canvas>
      <div style={{ position: 'absolute', bottom: 16, right: 16 }}>
        <ColorScale minValue={minGain} maxValue={maxGain} />
      </div>
    </div>
  );
};

export default PatternViewer;
