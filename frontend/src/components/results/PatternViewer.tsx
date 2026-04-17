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
 *
 * Camera behaviour:
 *   - On first mount the camera auto-fits to the pattern's bounding sphere
 *     so the whole diagram fills the viewport.
 *   - Camera position and orbit target are persisted in the Zustand store
 *     so they survive tab switches (the Canvas unmounts when the tab hides).
 */
import React, { useMemo, useState, useRef, useEffect } from 'react';
import { Canvas, useThree, useFrame } from '@react-three/fiber';
import { OrbitControls } from '@react-three/drei';
import * as THREE from 'three';
import { useAntennaStore } from '@/store/antennaStore';
import { sphericalToCartesian } from '@/utils/conversions';
import ColorScale from '@/components/common/ColorScale';

interface PatternMeshProps {
  opacity: number;
  wireframe: boolean;
}

/** Builds and renders the 3D radiation-pattern surface mesh. */
const PatternMesh: React.FC<PatternMeshProps> = ({ opacity, wireframe }) => {
  const simulationResult = useAntennaStore((s) => s.simulationResult);
  const groundType = useAntennaStore((s) => s.ground.type);

  const geometry = useMemo(() => {
    if (!simulationResult || simulationResult.pattern.length === 0) return null;

    const pattern = simulationResult.pattern;
    const SUPPRESSED_DB = -99;
    const hasGround = groundType !== 'free_space';
    const activeGains = pattern
      .filter((p) => p.gainDb > SUPPRESSED_DB && !(hasGround && p.theta > 90))
      .map((p) => p.gainDb);
    const minGain = activeGains.length > 0 ? Math.min(...activeGains) : -10;
    const maxGain = activeGains.length > 0 ? Math.max(...activeGains) : 0;
    const gainRange = maxGain - minGain || 1;

    const thetaSet = new Set<number>();
    const phiSet = new Set<number>();
    pattern.forEach((p) => {
      thetaSet.add(p.theta);
      phiSet.add(p.phi);
    });

    const thetas = Array.from(thetaSet).sort((a, b) => a - b);
    const phis = Array.from(phiSet).sort((a, b) => a - b);

    if (thetas.length < 2 || phis.length < 2) return null;

    const gainMap = new Map<string, number>();
    pattern.forEach((p) => {
      gainMap.set(`${p.theta},${p.phi}`, p.gainDb);
    });

    const positions: number[] = [];
    const colors: number[] = [];
    const indices: number[] = [];
    const suppressed: boolean[] = [];

    for (let ti = 0; ti < thetas.length; ti++) {
      for (let pi = 0; pi < phis.length; pi++) {
        const theta = thetas[ti];
        const phi = phis[pi];
        const gain = gainMap.get(`${theta},${phi}`) ?? minGain;

        const isBelowGround = gain <= SUPPRESSED_DB ||
          (groundType !== 'free_space' && theta > 90);

        if (isBelowGround) {
          positions.push(0, 0, 0);
          colors.push(0, 0, 0);
          suppressed.push(true);
        } else {
          const normalized = (gain - minGain) / gainRange;
          const r = Math.pow(10, (gain - minGain + 3) / 20) * 0.5;

          const { x, y, z } = sphericalToCartesian(r, theta, phi);
          positions.push(x, y, z);

          const color = new THREE.Color();
          color.setHSL((1 - normalized) * 0.66, 1.0, 0.5);
          colors.push(color.r, color.g, color.b);
          suppressed.push(false);
        }
      }
    }

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
      <meshStandardMaterial
        vertexColors
        side={THREE.DoubleSide}
        transparent
        opacity={opacity}
        depthWrite={opacity >= 0.99}
        wireframe={wireframe}
      />
    </mesh>
  );
};

// ────────────────────────────────────────────────────────────────────
// CameraController: auto-fits on first mount, restores persisted
// camera on remount, and saves camera state on every frame so it
// survives tab switches.
// ────────────────────────────────────────────────────────────────────

/** Compute the bounding sphere of the pattern mesh vertices. */
function patternBoundingSphere(
  pattern: { theta: number; phi: number; gainDb: number }[],
  groundType: string,
): { center: THREE.Vector3; radius: number } {
  const SUPPRESSED_DB = -99;
  const hasGround = groundType !== 'free_space';

  // Compute the minimum active gain once (not per-point).
  const activeGains = pattern
    .filter((pt) => pt.gainDb > SUPPRESSED_DB && !(hasGround && pt.theta > 90))
    .map((pt) => pt.gainDb);
  const minGain = activeGains.length > 0 ? Math.min(...activeGains) : -10;

  let maxR = 0;
  for (const p of pattern) {
    if (p.gainDb <= SUPPRESSED_DB) continue;
    if (hasGround && p.theta > 90) continue;
    const r = Math.pow(10, (p.gainDb - minGain + 3) / 20) * 0.5;
    if (r > maxR) maxR = r;
  }

  return { center: new THREE.Vector3(0, 0, 0), radius: maxR || 2 };
}

interface CameraControllerProps {
  pattern: { theta: number; phi: number; gainDb: number }[];
  groundType: string;
}

const CameraController: React.FC<CameraControllerProps> = ({ pattern, groundType }) => {
  const { camera } = useThree();
  const controlsRef = useRef<any>(null);
  const savedCamera = useAntennaStore((s) => s.patternCamera);
  const setPatternCamera = useAntennaStore((s) => s.setPatternCamera);
  const initialised = useRef(false);

  // On mount: either restore saved camera or auto-fit to pattern.
  useEffect(() => {
    if (initialised.current) return;
    initialised.current = true;

    if (savedCamera) {
      // Restore persisted position and target.
      camera.position.set(...savedCamera.position);
      if (controlsRef.current) {
        controlsRef.current.target.set(...savedCamera.target);
        controlsRef.current.update();
      }
    } else {
      // First-ever open: auto-fit to the pattern bounding sphere.
      const { center, radius } = patternBoundingSphere(pattern, groundType);
      // Position the camera so the sphere fills ~70% of the viewport.
      // For a perspective camera with fov F, distance d = r / sin(F/2).
      const fov = (camera as THREE.PerspectiveCamera).fov ?? 50;
      const dist = (radius / Math.sin((fov / 2) * Math.PI / 180)) * 1.15;
      // Place camera at a 45° elevation, looking at the center.
      const angle = Math.PI / 4;
      camera.position.set(
        center.x + dist * Math.cos(angle) * 0.7,
        center.y + dist * Math.sin(angle),
        center.z + dist * Math.cos(angle) * 0.7,
      );
      if (controlsRef.current) {
        controlsRef.current.target.copy(center);
        controlsRef.current.update();
      }
    }
  }, []);  // Run once on mount only.

  // Save camera state every ~10 frames so tab-switch preserves it.
  const frameCount = useRef(0);
  useFrame(() => {
    frameCount.current++;
    if (frameCount.current % 10 !== 0) return;
    const pos = camera.position;
    const tgt = controlsRef.current?.target;
    if (tgt) {
      setPatternCamera({
        position: [pos.x, pos.y, pos.z],
        target: [tgt.x, tgt.y, tgt.z],
      });
    }
  });

  return <OrbitControls ref={controlsRef} makeDefault />;
};


/** Outer component: shows the 3D pattern canvas with a colour-scale legend overlay. */
const PatternViewer: React.FC = () => {
  const simulationResult = useAntennaStore((s) => s.simulationResult);
  const groundType = useAntennaStore((s) => s.ground.type);
  const [opacity, setOpacity] = useState(0.6);
  const [wireframe, setWireframe] = useState(false);
  const [showGround, setShowGround] = useState(true);

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
        <PatternMesh opacity={opacity} wireframe={wireframe} />
        {showGround && groundType !== 'free_space' && (
          <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, 0, 0]}>
            <planeGeometry args={[12, 12]} />
            <meshStandardMaterial color="#556655" transparent opacity={0.2} side={THREE.DoubleSide} />
          </mesh>
        )}
        <axesHelper args={[3]} />
        <CameraController pattern={simulationResult.pattern} groundType={groundType} />
      </Canvas>
      <div className="pattern-controls" style={{ position: 'absolute', top: 12, left: 12 }}>
        <label className="pattern-control-row">
          Opacity
          <input
            type="range"
            min={0.1}
            max={1}
            step={0.05}
            value={opacity}
            onChange={(e) => setOpacity(parseFloat(e.target.value))}
          />
          <span className="muted small">{opacity.toFixed(2)}</span>
        </label>
        <label className="pattern-control-row">
          <input
            type="checkbox"
            checked={wireframe}
            onChange={(e) => setWireframe(e.target.checked)}
          />
          Wireframe
        </label>
        <label className="pattern-control-row">
          <input
            type="checkbox"
            checked={showGround}
            onChange={(e) => setShowGround(e.target.checked)}
          />
          Show ground
        </label>
      </div>
      <div style={{ position: 'absolute', bottom: 16, right: 16 }}>
        <ColorScale minValue={minGain} maxValue={maxGain} />
      </div>
    </div>
  );
};

export default PatternViewer;
