/**
 * 3D wire antenna editor using Three.js via react-three-fiber.
 *
 * Renders each wire as a cylinder between its two endpoints, with spheres at
 * the nodes.  A grid, axes helper, and optional ground plane are shown.
 *
 * Coordinate mapping: The physics model uses Z-up, but Three.js uses Y-up.
 * The `physicsToThree` helper swaps Y<->Z so the 3D scene displays correctly.
 * Cylinders are created along the Y axis then rotated via quaternion to align
 * with the wire direction.
 */
import React, { useRef, useMemo } from 'react';
import { Canvas } from '@react-three/fiber';
import { OrbitControls, Line } from '@react-three/drei';
import * as THREE from 'three';
import { useAntennaStore } from '@/store/antennaStore';
import { physicsToThree } from '@/utils/conversions';
import type { Wire } from '@/types';

interface WireMeshProps {
  wire: Wire;
  isSelected: boolean;
  onClick: () => void;
}

/**
 * Renders a single wire as a cylinder with endpoint spheres.
 * The cylinder is placed at the midpoint and oriented via a quaternion that
 * rotates the default Y-axis cylinder to match the wire direction.
 */
const WireMesh: React.FC<WireMeshProps> = ({ wire, isSelected, onClick }) => {
  const meshRef = useRef<THREE.Mesh>(null);

  // Compute midpoint, orientation quaternion, and length from the two endpoints
  const { position, quaternion, length } = useMemo(() => {
    const s = physicsToThree(wire.x1, wire.y1, wire.z1);
    const e = physicsToThree(wire.x2, wire.y2, wire.z2);
    const start = new THREE.Vector3(...s);
    const end = new THREE.Vector3(...e);
    const mid = new THREE.Vector3().addVectors(start, end).multiplyScalar(0.5);
    const dir = new THREE.Vector3().subVectors(end, start);
    const len = dir.length();
    dir.normalize();

    const quat = new THREE.Quaternion();
    const cylAxis = new THREE.Vector3(0, 1, 0);
    quat.setFromUnitVectors(cylAxis, dir);

    return { position: mid, quaternion: quat, length: len };
  }, [wire.x1, wire.y1, wire.z1, wire.x2, wire.y2, wire.z2]);

  // Scale up the display radius so thin wires remain visible in the viewport
  const displayRadius = Math.max(wire.radius * 50, 0.05);

  return (
    <group>
      <mesh
        ref={meshRef}
        position={position}
        quaternion={quaternion}
        onClick={(e) => {
          e.stopPropagation();
          onClick();
        }}
      >
        <cylinderGeometry args={[displayRadius, displayRadius, length, 8]} />
        <meshStandardMaterial
          color={isSelected ? '#ffdd00' : '#4488ff'}
          emissive={isSelected ? '#443300' : '#001133'}
        />
      </mesh>
      {/* Endpoint spheres */}
      <mesh position={physicsToThree(wire.x1, wire.y1, wire.z1)}>
        <sphereGeometry args={[displayRadius * 1.5, 8, 8]} />
        <meshStandardMaterial color={isSelected ? '#ffaa00' : '#66aaff'} />
      </mesh>
      <mesh position={physicsToThree(wire.x2, wire.y2, wire.z2)}>
        <sphereGeometry args={[displayRadius * 1.5, 8, 8]} />
        <meshStandardMaterial color={isSelected ? '#ffaa00' : '#66aaff'} />
      </mesh>
    </group>
  );
};

/** Semi-transparent ground plane at Y=0 (physics Z=0), shown when ground is not free-space. */
const GroundPlane: React.FC = () => {
  return (
    <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, 0, 0]}>
      <planeGeometry args={[40, 40]} />
      <meshStandardMaterial color="#224422" transparent opacity={0.3} side={THREE.DoubleSide} />
    </mesh>
  );
};

/** Inner scene: lights, grid, axes, ground plane, and all wire meshes.
 *  Axis lines show physics axes: X (red), Y (green, into screen), Z (blue, up).
 */
const SceneContent: React.FC = () => {
  const { wires, selectedWireId, selectWire, ground } = useAntennaStore();

  return (
    <>
      <ambientLight intensity={0.5} />
      <directionalLight position={[10, 10, 10]} intensity={0.8} />
      <axesHelper args={[5]} />
      <gridHelper args={[20, 20, '#333355', '#222244']} />
      {ground.type !== 'free_space' && <GroundPlane />}

      {wires.map((wire) => (
        <WireMesh
          key={wire.id}
          wire={wire}
          isSelected={selectedWireId === wire.id}
          onClick={() => selectWire(wire.id)}
        />
      ))}

      {/* Axis lines: X (red), Y (green, into screen), Z (blue, up) */}
      <Line
        points={[[0, 0, 0], [6, 0, 0]]}
        color="#ff4444"
        lineWidth={1}
      />
      <Line
        points={[[0, 0, 0], [0, 0, 6]]}
        color="#44ff44"
        lineWidth={1}
      />
      <Line
        points={[[0, 0, 0], [0, 6, 0]]}
        color="#4444ff"
        lineWidth={1}
      />

      <OrbitControls makeDefault />
    </>
  );
};

/** Top-level editor component: wraps the Three.js Canvas with orbit controls. */
const WireEditor: React.FC = () => {
  return (
    <div className="editor-container">
      <Canvas camera={{ position: [10, 10, 10], fov: 50, near: 0.01, far: 10000 }}>
        <SceneContent />
      </Canvas>
    </div>
  );
};

export default WireEditor;
