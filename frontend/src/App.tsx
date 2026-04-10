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
