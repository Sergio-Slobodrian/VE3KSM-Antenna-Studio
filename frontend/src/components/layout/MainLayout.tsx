/**
 * Main layout with a resizable split panel.
 *
 * Left panel: scrollable input forms (wires, source, loads, ground,
 * frequency).  Right panel: tabbed viewer switching between 3D editor,
 * radiation pattern, polar cuts, Smith chart, metrics, SWR chart,
 * impedance chart, current distribution, and matching network.
 *
 * A non-blocking warnings banner appears above the tab content whenever
 * the segmentation validator emitted any findings.
 */
import React, { useState, useCallback, useRef, useEffect } from 'react';
import WireTable from '@/components/input/WireTable';
import SourceConfig from '@/components/input/SourceConfig';
import GroundConfig from '@/components/input/GroundConfig';
import FrequencyInput from '@/components/input/FrequencyInput';
import EnvironmentConfig from '@/components/input/EnvironmentConfig';
import LoadEditor from '@/components/input/LoadEditor';
import TLEditor from '@/components/input/TLEditor';
import WireEditor from '@/components/editor/WireEditor';
import PatternViewer from '@/components/results/PatternViewer';
import PolarCut from '@/components/results/PolarCut';
import SmithChart from '@/components/results/SmithChart';
import MetricsPanel from '@/components/results/MetricsPanel';
import SWRChart from '@/components/results/SWRChart';
import ImpedanceChart from '@/components/results/ImpedanceChart';
import CurrentDisplay from '@/components/results/CurrentDisplay';
import MatchingNetwork from '@/components/results/MatchingNetwork';
import NearFieldViewer from '@/components/results/NearFieldViewer';
import CMAViewer from '@/components/results/CMAViewer';
import OptimizerViewer from '@/components/results/OptimizerViewer';
import ParetoViewer from '@/components/results/ParetoViewer';
import TransientViewer from '@/components/results/TransientViewer';
import PolarizationViewer from '@/components/results/PolarizationViewer';
import ConvergenceViewer from '@/components/results/ConvergenceViewer';
import WarningsBanner from '@/components/results/WarningsBanner';
import SweepExport from '@/components/results/SweepExport';

type Tab =
  | '3d'
  | 'pattern'
  | 'cuts'
  | 'smith'
  | 'metrics'
  | 'swr'
  | 'impedance'
  | 'currents'
  | 'matching'
  | 'nearfield'
  | 'polarization'
  | 'cma'
  | 'optimizer'
  | 'pareto'
  | 'transient'
  | 'convergence';

const MIN_PANEL_WIDTH = 200;
const MAX_PANEL_WIDTH = 800;
const DEFAULT_PANEL_WIDTH = 380;

const MainLayout: React.FC = () => {
  const [activeTab, setActiveTab] = useState<Tab>('3d');
  const [panelWidth, setPanelWidth] = useState(DEFAULT_PANEL_WIDTH);
  const [collapsed, setCollapsed] = useState(false);
  const isDragging = useRef(false);
  const startX = useRef(0);
  const startWidth = useRef(0);

  const onMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    isDragging.current = true;
    startX.current = e.clientX;
    startWidth.current = panelWidth;
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
  }, [panelWidth]);

  useEffect(() => {
    const onMouseMove = (e: MouseEvent) => {
      if (!isDragging.current) return;
      const delta = e.clientX - startX.current;
      const newWidth = Math.max(MIN_PANEL_WIDTH, Math.min(MAX_PANEL_WIDTH, startWidth.current + delta));
      setPanelWidth(newWidth);
    };
    const onMouseUp = () => {
      if (!isDragging.current) return;
      isDragging.current = false;
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
    };
    window.addEventListener('mousemove', onMouseMove);
    window.addEventListener('mouseup', onMouseUp);
    return () => {
      window.removeEventListener('mousemove', onMouseMove);
      window.removeEventListener('mouseup', onMouseUp);
    };
  }, []);

  const toggleCollapsed = () => setCollapsed((c) => !c);

  const tabs: { id: Tab; label: string }[] = [
    { id: '3d', label: '3D Editor' },
    { id: 'pattern', label: '3D Pattern' },
    { id: 'cuts', label: 'Polar Cuts' },
    { id: 'smith', label: 'Smith' },
    { id: 'metrics', label: 'Metrics' },
    { id: 'swr', label: 'SWR' },
    { id: 'impedance', label: 'Impedance' },
    { id: 'currents', label: 'Currents' },
    { id: 'matching', label: 'Matching' },
    { id: 'nearfield', label: 'Near-Field' },
    { id: 'polarization', label: 'Polarization' },
    { id: 'cma', label: 'CMA' },
    { id: 'optimizer', label: 'Optimizer' },
    { id: 'pareto', label: 'Pareto' },
    { id: 'transient', label: 'Transient' },
    { id: 'convergence', label: 'Convergence' },
  ];

  const renderTabContent = () => {
    switch (activeTab) {
      case '3d': return <WireEditor />;
      case 'pattern': return <PatternViewer />;
      case 'cuts': return <PolarCut />;
      case 'smith': return <SmithChart />;
      case 'metrics': return <MetricsPanel />;
      case 'swr': return <SWRChart />;
      case 'impedance': return <ImpedanceChart />;
      case 'currents': return <CurrentDisplay />;
      case 'matching': return <MatchingNetwork />;
      case 'nearfield': return <NearFieldViewer />;
      case 'polarization': return <PolarizationViewer />;
      case 'cma': return <CMAViewer />;
      case 'optimizer': return <OptimizerViewer />;
      case 'pareto': return <ParetoViewer />;
      case 'transient': return <TransientViewer />;
      case 'convergence': return <ConvergenceViewer />;
    }
  };

  return (
    <div className="main-layout">
      {!collapsed && (
        <div className="left-panel" style={{ width: panelWidth }}>
          <div className="panel-scroll">
            <WireTable />
            <SourceConfig />
            <LoadEditor />
            <TLEditor />
            <GroundConfig />
            <EnvironmentConfig />
            <FrequencyInput />
          </div>
        </div>
      )}
      <div className="panel-divider">
        {!collapsed && (
          <div className="resize-handle" onMouseDown={onMouseDown} />
        )}
        <button
          className="panel-toggle-btn"
          onClick={toggleCollapsed}
          title={collapsed ? 'Show input panel' : 'Hide input panel'}
        >
          {collapsed ? '\u25B6' : '\u25C0'}
        </button>
      </div>
      <div className="right-panel">
        <WarningsBanner />
        <SweepExport />
        <div className="tab-bar">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              className={`tab-btn ${activeTab === tab.id ? 'tab-active' : ''}`}
              onClick={() => setActiveTab(tab.id)}
            >
              {tab.label}
            </button>
          ))}
        </div>
        <div className="tab-content">{renderTabContent()}</div>
      </div>
    </div>
  );
};

export default MainLayout;
