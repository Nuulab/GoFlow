import { Outlet } from 'react-router-dom'
import { Sidebar } from './Sidebar'

export default function Layout() {
  return (
    <div className="flex h-screen bg-background overflow-hidden font-sans">
      <Sidebar />

      {/* Main content */}
      <main className="flex-1 overflow-auto bg-background/50 relative">
        {/* Subtle background gradient/noise could go here */}
        <div className="absolute inset-0 bg-gradient-to-br from-primary/5 via-transparent to-transparent pointer-events-none opacity-50" />
        
        <div className="relative h-full">
            <Outlet />
        </div>
      </main>
    </div>
  )
}
