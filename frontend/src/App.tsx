/**
 * Root application component.
 *
 * Composes the three top-level layout regions: Header (toolbar with simulate
 * buttons), MainLayout (resizable split between input panel and tabbed
 * viewer), and StatusBar (impedance/SWR summary or error display).
 */
import React from 'react';
import Header from '@/components/layout/Header';
import MainLayout from '@/components/layout/MainLayout';
import StatusBar from '@/components/layout/StatusBar';

const App: React.FC = () => {
  return (
    <div className="app">
      <Header />
      <MainLayout />
      <StatusBar />
    </div>
  );
};

export default App;
