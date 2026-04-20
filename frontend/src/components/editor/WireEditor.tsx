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
 * 3D wire antenna editor using Three.js via react-three-fiber.
 *
 * Renders each wire as a cylinder between its two endpoints, with draggable
 * spheres at the nodes. Dragging uses @react-three/drei's DragControls for
 * reliable pointer tracking. The wire geometry is updated only on drag end
 * to avoid per-frame store updates. Orbit controls are disabled while dragging.
 *
 * Coordinate mapping: physics Z-up → Three.js Y-up via physicsToThree/threeToPhysics.
 */
import React, { useRef, useMemo, useState, useCallback, useEffect } from 'react';
import { Canvas, useThree } from '@react-three/fiber';
import { OrbitControls, Line } from '@react-three/drei';
import * as THREE from 'three';
import { useAntennaStore } from '@/store/antennaStore';
import { physicsToThree } from '@/utils/conversions';
import type { Wire } from '@/types';

/** Convert Three.js Y-up coords back to physics Z-up. */
function threeToPhysics(tx: number, ty: number, tz: number): [number, number, number] {
  return [tx, tz, ty];
}

// ── Draggable Endpoint ───────────────────────────────────────────────────────

interface DragHandleProps {
  initialPosition: [number, number, number];
  color: string;
  radius: number;
  wireId: string;
  endpoint: 'start' | 'end';
  onDragStart: () => void;
  onDragEnd: () => void;
}

/**
 * Draggable sphere at a wire endpoint. Uses simple pointer-down/move/up on the
 * canvas with a drag plane perpendicular to the camera. Updates are batched:
 * only a local Three.js position changes during drag; the Zustand store is
 * written once on pointer-up.
 */
const DragHandle: React.FC<DragHandleProps> = ({
  initialPosition, color, radius, wireId, endpoint, onDragStart, onDragEnd,
}) => {
  const meshRef = useRef<THREE.Mesh>(null);
  const dragging = useRef(false);
  const plane = useRef(new THREE.Plane());
  const startWorld = useRef(new THREE.Vector3());
  const startMeshPos = useRef(new THREE.Vector3());
  const intersect = useRef(new THREE.Vector3());
  const { camera, gl } = useThree();

  // Sync mesh position when the store-driven prop changes (e.g., after load/template)
  useEffect(() => {
    if (meshRef.current && !dragging.current) {
      meshRef.current.position.set(...initialPosition);
    }
  }, [initialPosition]);

  const getPointerNDC = useCallback((e: PointerEvent): THREE.Vector2 => {
    const rect = gl.domElement.getBoundingClientRect();
    return new THREE.Vector2(
      ((e.clientX - rect.left) / rect.width) * 2 - 1,
      -((e.clientY - rect.top) / rect.height) * 2 + 1,
    );
  }, [gl]);

  const onPointerDown = useCallback((e: any) => {
    e.stopPropagation();
    const pe = e.nativeEvent ?? e as PointerEvent;
    dragging.current = true;
    onDragStart();
    gl.domElement.setPointerCapture(pe.pointerId);

    // Set up a drag plane through the mesh position, facing the camera
    const camDir = new THREE.Vector3();
    camera.getWorldDirection(camDir);
    const meshPos = meshRef.current!.position.clone();
    plane.current.setFromNormalAndCoplanarPoint(camDir, meshPos);

    // Record where the ray hits the plane (anchor) and the mesh position
    const ndc = getPointerNDC(pe);
    const ray = new THREE.Raycaster();
    ray.setFromCamera(ndc, camera);
    ray.ray.intersectPlane(plane.current, startWorld.current);
    startMeshPos.current.copy(meshPos);
  }, [camera, gl, onDragStart, getPointerNDC]);

  const onPointerMove = useCallback((e: any) => {
    if (!dragging.current) return;
    e.stopPropagation();
    const pe = e.nativeEvent ?? e as PointerEvent;

    const ndc = getPointerNDC(pe);
    const ray = new THREE.Raycaster();
    ray.setFromCamera(ndc, camera);
    if (ray.ray.intersectPlane(plane.current, intersect.current)) {
      // Delta from the initial hit to the current hit
      const delta = intersect.current.clone().sub(startWorld.current);
      const newPos = startMeshPos.current.clone().add(delta);
      // Move the mesh locally (no store update yet)
      meshRef.current!.position.copy(newPos);
    }
  }, [camera, getPointerNDC]);

  const onPointerUp = useCallback((e: any) => {
    if (!dragging.current) return;
    e.stopPropagation();
    dragging.current = false;
    const pe = e.nativeEvent ?? e as PointerEvent;
    gl.domElement.releasePointerCapture(pe.pointerId);

    // Commit the final position to the store
    const p = meshRef.current!.position;
    const [px, py, pz] = threeToPhysics(p.x, p.y, p.z);
    const updates: Partial<Wire> = endpoint === 'start'
      ? { x1: px, y1: py, z1: pz }
      : { x2: px, y2: py, z2: pz };
    useAntennaStore.getState().updateWire(wireId, updates);
    onDragEnd();
  }, [gl, wireId, endpoint, onDragEnd]);

  return (
    <mesh
      ref={meshRef}
      position={initialPosition}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
    >
      <sphereGeometry args={[radius, 12, 12]} />
      <meshStandardMaterial
        color={color}
        transparent
        opacity={0.85}
      />
    </mesh>
  );
};

// ── Wire Mesh ────────────────────────────────────────────────────────────────

interface WireMeshProps {
  wire: Wire;
  isSelected: boolean;
  onClick: () => void;
  onDragStart: () => void;
  onDragEnd: () => void;
}

const WireMesh: React.FC<WireMeshProps> = ({ wire, isSelected, onClick, onDragStart, onDragEnd }) => {
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
    quat.setFromUnitVectors(new THREE.Vector3(0, 1, 0), dir);

    return { position: mid, quaternion: quat, length: len };
  }, [wire.x1, wire.y1, wire.z1, wire.x2, wire.y2, wire.z2]);

  const displayRadius = Math.max(wire.radius * 50, 0.05);
  const handleRadius = displayRadius * 2.5;

  return (
    <group>
      <mesh
        position={position}
        quaternion={quaternion}
        onClick={(e) => { e.stopPropagation(); onClick(); }}
      >
        <cylinderGeometry args={[displayRadius, displayRadius, length, 8]} />
        <meshStandardMaterial
          color={isSelected ? '#ffdd00' : '#4488ff'}
          emissive={isSelected ? '#443300' : '#001133'}
        />
      </mesh>
      <DragHandle
        initialPosition={physicsToThree(wire.x1, wire.y1, wire.z1)}
        color={isSelected ? '#ffaa00' : '#66aaff'}
        radius={handleRadius}
        wireId={wire.id}
        endpoint="start"
        onDragStart={onDragStart}
        onDragEnd={onDragEnd}
      />
      <DragHandle
        initialPosition={physicsToThree(wire.x2, wire.y2, wire.z2)}
        color={isSelected ? '#ffaa00' : '#66aaff'}
        radius={handleRadius}
        wireId={wire.id}
        endpoint="end"
        onDragStart={onDragStart}
        onDragEnd={onDragEnd}
      />
    </group>
  );
};

// ── Ground Plane ─────────────────────────────────────────────────────────────

const GroundPlane: React.FC = () => (
  <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, 0, 0]}>
    <planeGeometry args={[40, 40]} />
    <meshStandardMaterial color="#224422" transparent opacity={0.3} side={THREE.DoubleSide} />
  </mesh>
);

// ── Scene ────────────────────────────────────────────────────────────────────

const SceneContent: React.FC = () => {
  const { wires, selectedWireId, selectWire, ground } = useAntennaStore();
  const controlsRef = useRef<any>(null);

  const handleDragStart = useCallback(() => {
    if (controlsRef.current) controlsRef.current.enabled = false;
  }, []);

  const handleDragEnd = useCallback(() => {
    if (controlsRef.current) controlsRef.current.enabled = true;
  }, []);

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
          onDragStart={handleDragStart}
          onDragEnd={handleDragEnd}
        />
      ))}

      <Line points={[[0, 0, 0], [6, 0, 0]]} color="#ff4444" lineWidth={1} />
      <Line points={[[0, 0, 0], [0, 0, 6]]} color="#44ff44" lineWidth={1} />
      <Line points={[[0, 0, 0], [0, 6, 0]]} color="#4444ff" lineWidth={1} />

      <OrbitControls ref={controlsRef} makeDefault />
    </>
  );
};

// ── Canvas ───────────────────────────────────────────────────────────────────

const WireEditor: React.FC = () => (
  <div className="editor-container">
    <Canvas camera={{ position: [10, 10, 10], fov: 50, near: 0.01, far: 10000 }}>
      <SceneContent />
    </Canvas>
  </div>
);

export default WireEditor;
