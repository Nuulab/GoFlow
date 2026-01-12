import { cn } from '@/lib/utils'
import {
  Activity,
  AlertTriangle,
  GitBranch,
  LayoutDashboard,
  ListTodo,
  Search,
  Settings
} from 'lucide-react'
import { NavLink } from 'react-router-dom'

const navItems = [
  { to: '/', icon: LayoutDashboard, label: 'Dashboard' },
  { to: '/jobs', icon: ListTodo, label: 'Jobs' },
  { to: '/workflows', icon: GitBranch, label: 'Workflows' },
  { to: '/events', icon: Activity, label: 'Events' },
  { to: '/dlq', icon: AlertTriangle, label: 'Dead Letter Queue' },
]

export function Sidebar() {
  return (
    <aside className="w-64 border-r border-sidebar-border bg-sidebar flex flex-col h-full transition-all duration-300">
      {/* Header */}
      <div className="h-14 flex items-center px-4 border-b border-sidebar-border/50">
        <div className="flex items-center gap-2">
          <img 
            src="/goflow-logo.png" 
            alt="GoFlow" 
            className="w-8 h-10 object-contain rounded"
            style={{ aspectRatio: '1198/1494' }}
          />
          <span className="font-semibold text-sidebar-foreground tracking-tight bg-gradient-to-r from-[#5B8DEE] to-[#7C5BBF] bg-clip-text text-transparent">GoFlow</span>
        </div>
      </div>

      {/* Search (Visual Only for now) */}
      <div className="px-3 py-3">
        <div className="relative">
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-sidebar-foreground/40" />
          <input 
            className="w-full bg-sidebar-accent/50 text-sm pl-9 pr-3 py-2 rounded-md border border-transparent focus:border-sidebar-ring focus:bg-sidebar-accent outline-none text-sidebar-foreground placeholder:text-sidebar-foreground/40 transition-all font-medium"
            placeholder="Search..."
          />
        </div>
      </div>
      
      {/* Nav */}
      <nav className="flex-1 px-3 space-y-1 overflow-y-auto py-2">
        <div className="px-2 py-1.5 text-xs font-semibold text-sidebar-foreground/50 uppercase tracking-wider">Platform</div>
        {navItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.to === '/'}
            className={({ isActive }) =>
              cn(
                'flex items-center gap-3 px-3 py-2 rounded-md text-sm font-medium transition-all group',
                isActive
                  ? 'bg-sidebar-accent text-sidebar-accent-foreground shadow-sm'
                  : 'text-sidebar-foreground/70 hover:bg-sidebar-accent/50 hover:text-sidebar-foreground'
              )
            }
          >
            {({ isActive }) => (
                <>
                    <item.icon className={cn(
                        "w-4 h-4 transition-colors",
                        isActive ? "text-primary" : "text-sidebar-foreground/50 group-hover:text-sidebar-foreground"
                    )} />
                    {item.label}
                    {isActive && (
                        <div className="ml-auto w-1.5 h-1.5 rounded-full bg-primary shadow-[0_0_8px_hsl(var(--primary))]"></div>
                    )}
                </>
            )}
          </NavLink>
        ))}
      </nav>
      
      {/* Footer / User */}
      <div className="p-3 border-t border-sidebar-border/50">
        <button className="flex items-center gap-3 w-full p-2 rounded-md hover:bg-sidebar-accent/50 transition-colors group">
            <div className="w-8 h-8 rounded-full bg-gradient-to-tr from-primary to-purple-500 flex items-center justify-center text-[10px] font-bold text-white shadow-lg">
                JD
            </div>
            <div className="flex-1 text-left">
                <div className="text-sm font-medium text-sidebar-foreground">John Doe</div>
                <div className="text-xs text-sidebar-foreground/50">Admin Workspace</div>
            </div>
            <Settings className="w-4 h-4 text-sidebar-foreground/40 group-hover:text-sidebar-foreground transition-colors" />
        </button>
      </div>
    </aside>
  )
}
