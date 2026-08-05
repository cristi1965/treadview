import React from 'react';
import { Sidebar } from './Sidebar';
import { Header } from './Header';

interface LayoutProps {
  children: React.ReactNode;
  title?: string;
}

export const Layout: React.FC<LayoutProps> = ({ children, title }) => {
  return (
    <div className="flex min-h-screen bg-base text-ink">
      <Sidebar />
      
      <div className="flex-1 flex flex-col min-w-0">
        <Header title={title} />
        
        <main className="flex-1 p-6">
          <div className="mx-auto max-w-[1480px]">
            {children}
          </div>
        </main>
      </div>
    </div>
  );
};
