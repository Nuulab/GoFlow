import { Route, Routes } from 'react-router-dom'
import Layout from './components/Layout'
import Dashboard from './pages/Dashboard'
import DLQ from './pages/DLQ'
import Events from './pages/Events'
import Jobs from './pages/Jobs'
import Workflows from './pages/Workflows'

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<Layout />}>
        <Route index element={<Dashboard />} />
        <Route path="jobs" element={<Jobs />} />
        <Route path="workflows" element={<Workflows />} />
        <Route path="events" element={<Events />} />
        <Route path="dlq" element={<DLQ />} />
      </Route>
    </Routes>
  )
}
