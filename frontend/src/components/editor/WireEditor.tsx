import React, { useRef, useMemo } from 'react';
import { Canvas } from '@react-three/fiber';
import { OrbitControls, Line } from '@react-three/drei';
import * as THREE from 'three';
import { useAntennaStore } from '@/store/antennaStore';
import type { Wire } from '@/types';

interface WireMeshProps {
  wire: Wire;
  isSelected: boolean;
  onClick: () => void;
}

const WireMesh: React.FC<WireMeshProps> = ({ wire, isSelected, onClick }) => {
  const meshRef = useRef<THREE.Mesh>(null);

  const { position, quaternion, length } = useMemo(() => {
    const start = new THREE.Vector3(wire.x1, wire.y1, wire.z1);
    const end = new THREE.Vector3(wire.x2, wire.y2, wire.z2);
    const mid = new THREE.Vector3().addVectors(start, end).multiplyScalar(0.5);
    const dir = new THREE.Vector3().subVectors(end, start);
    const len = dir.length();
    dir.normalize();

    const quat = new THREE.Quaternion();
    const up = new THREE.Vector3(0, 1, 0);
    quat.setFromUnitVectors(up, dir);

    return { position: mid, quaternion: quat, length: len };
  }, [wire.x1, wire.y1, wire.z1, wire.x2, wire.y2, wire.z2]);

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
      <mesh position={[wire.x1, wire.y1, wire.z1]}>
        <sphereGeometry args={[displayRadius * 1.5, 8, 8]} />
        <meshStandardMaterial color={isSelected ? '#ffaa00' : '#66aaff'} />
      </mesh>
      <mesh position={[wire.x2, wire.y2, wire.z2]}>
        <sphereGeometry args={[displayRadius * 1.5, 8, 8]} />
        <meshStandardMaterial color={isSelected ? '#ffaa00' : '#66aaff'} />
      </mesh>
    </group>
  );
};

const GroundPlane: React.FC = () => {
  return (
    <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, 0, 0]}>
      <planeGeometry args={[40, 40]} />
      <meshStandardMaterial color="#224422" transparent opacity={0.3} side={THREE.DoubleSide} />
    </mesh>
  );
};

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

      {/* Axis labels using Line for reference */}
      <Line
        points={[
          [0, 0, 0],
          [6, 0, 0],
        ]}
        color="#ff4444"
        lineWidth={1}
      />
      <Line
        points={[
          [0, 0, 0],
          [0, 6, 0],
        ]}
        color="#44ff44"
        lineWidth={1}
      />
      <Line
        points={[
          [0, 0, 0],
          [0, 0, 6],
        ]}
        color="#4444ff"
        lineWidth={1}
      />

      <OrbitControls makeDefault />
    </>
  );
};

const WireEditor: React.FC = () => {
  return (
    <div className="editor-container">
      <Canvas camera={{ position: [10, 10, 10], fov: 50 }}>
        <SceneContent />
      </Canvas>
    </div>
  );
};

export default WireEditor;
