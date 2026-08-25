import { BrowserRouter, Route, Routes } from 'react-router-dom'
import { useAuth } from './AuthContext'
import { JobDetail } from './JobDetail'
import { JobList } from './JobList'

function Shell() {
  const { token, error, queueId } = useAuth()

  return (
    <div style={{ padding: '1.5rem', maxWidth: '56rem' }}>
      <h1 style={{ marginTop: 0 }}>Chronos</h1>
      {error && <p style={{ color: 'crimson' }}>Auth error: {error}</p>}
      {!error && !token && <p>Loading token…</p>}
      {token && (
        <>
          <p style={{ color: '#666', fontSize: '0.9rem' }}>
            Authenticated · queue {queueId}
          </p>
          <Routes>
            <Route path="/" element={<JobList />} />
            <Route path="/jobs/:id" element={<JobDetail />} />
          </Routes>
        </>
      )}
    </div>
  )
}

function App() {
  return (
    <BrowserRouter>
      <Shell />
    </BrowserRouter>
  )
}

export default App
