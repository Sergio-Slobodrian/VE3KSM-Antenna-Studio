import React, { useMemo } from 'react';
import { Canvas } from '@react-three/fiber';
import { OrbitControls } from '@react-three/drei';
import * as THREE from 'three';
import { useAntennaStore } from '@/store/antennaStore';
import { sphericalToCartesian } from '@/utils/conversions';
import ColorScale from '@/components/common/ColorScale';

const PatternMesh: React.FC = () => {
  const simulationResult = useAntennaStore((s) => s.simulationResult);

  const geometry = useMemo(() => {
    if (!simulationResult || simulationResult.pattern.length === 0) return null;

    const pattern = simulationResult.pattern;
    const gains = pattern.map((p) => p.gainDb);
    const minGain = Math.min(...gains);
    const maxGain = Math.max(...gains);
    const gainRange = maxGain - minGain || 1;

    // Build a map of theta/phi unique values
    const thetaSet = new Set<number>();
    const phiSet = new Set<number>();
    pattern.forEach((p) => {
      thetaSet.add(p.theta);
      phiSet.add(p.phi);
    });

    const thetas = Array.from(thetaSet).sort((a, b) => a - b);
    const phis = Array.from(phiSet).sort((a, b) => a - b);

    if (thetas.length < 2 || phis.length < 2) return null;

    // Build a lookup map
    const gainMap = new Map<string, number>();
    pattern.forEach((p) => {
      gainMap.set(`${p.theta},${p.phi}`, p.gainDb);
    });

    const positions: number[] = [];
    const colors: number[] = [];
    const indices: number[] = [];

    // Generate vertices
    for (let ti = 0; ti < thetas.length; ti++) {
      for (let pi = 0; pi < phis.length; pi++) {
        const theta = thetas[ti];
        const phi = phis[pi];
        const gain = gainMap.get(`${theta},${phi}`) ?? minGain;
        const normalized = (gain - minGain) / gainRange;
        const r = Math.pow(10, (gain - minGain + 3) / 20) * 0.5;

        const { x, y, z } = sphericalToCartesian(r, theta, phi);
        positions.push(x, y, z);

        // Color: blue (low) -> green -> yellow -> red (high)
        const color = new THREE.Color();
        color.setHSL((1 - normalized) * 0.66, 1.0, 0.5);
        colors.push(color.r, color.g, color.b);
      }
    }

    // Generate triangle indices
    for (let ti = 0; ti < thetas.length - 1; ti++) {
      for (let pi = 0; pi < phis.length - 1; pi++) {
        const a = ti * phis.length + pi;
        const b = a + 1;
        const c = (ti + 1) * phis.length + pi;
        const d = c + 1;

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
      <meshStandardMaterial vertexColors side={THREE.DoubleSide} />
    </mesh>
  );
};

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

  const gains = simulationResult.pattern.map((p) => p.gainDb);
  const minGain = Math.min(...gains);
  const maxGain = Math.max(...gains);

  return (
    <div className="editor-container" style={{ position: 'relative' }}>
      <Canvas camera={{ position: [5, 5, 5], fov: 50 }}>
        <ambientLight intensity={0.4} />
        <directionalLight position={[5, 10, 5]} intensity={0.7} />
        <PatternMesh />
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
