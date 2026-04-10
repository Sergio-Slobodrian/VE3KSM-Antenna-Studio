import React, { useState, useCallback, useRef, useEffect } from 'react';
import WireTable from '@/components/input/WireTable';
import SourceConfig from '@/components/input/SourceConfig';
import GroundConfig from '@/components/input/GroundConfig';
import FrequencyInput from '@/components/input/FrequencyInput';
import WireEditor from '@/components/editor/WireEditor';
import PatternViewer from '@/components/results/PatternViewer';
import SWRChart from '@/components/results/SWRChart';
import ImpedanceChart from '@/components/results/ImpedanceChart';
import CurrentDisplay from '@/components/results/CurrentDisplay';

type Tab = '3d' | 'pattern' | 'swr' | 'impedance' | 'currents';

const MIN_PANEL_WIDTH = 200;
const MAX_PANEL_WIDTH = 800;
const DEFAULT_PANEL_WIDTH = 350;

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
    { id: 'pattern', label: 'Radiation Pattern' },
    { id: 'swr', label: 'SWR' },
    { id: 'impedance', label: 'Impedance' },
    { id: 'currents', label: 'Currents' },
  ];

  const renderTabContent = () => {
    switch (activeTab) {
      case '3d':
        return <WireEditor />;
      case 'pattern':
        return <PatternViewer />;
      case 'swr':
        return <SWRChart />;
      case 'impedance':
        return <ImpedanceChart />;
      case 'currents':
        return <CurrentDisplay />;
    }
  };

  return (
    <div className="main-layout">
      {!collapsed && (
        <div className="left-panel" style={{ width: panelWidth }}>
          <div className="panel-scroll">
            <WireTable />
            <SourceConfig />
            <GroundConfig />
            <FrequencyInput />
          </div>
        </div>
      )}
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
      <div className="right-panel">
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
