import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { listJobs, type Job } from './api'
import { useAuth } from './AuthContext'

const STATES = [
  '',
  'pending',
  'runnable',
  'running',
  'succeeded',
  'failed_retrying',
  'dead_lettered',
  'canceled',
] as const

export function JobList() {
  const { token, queueId } = useAuth()
  const navigate = useNavigate()
  const [jobs, setJobs] = useState<Job[]>([])
  const [stateFilter, setStateFilter] = useState('')
  const [loadError, setLoadError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const load = useCallback(async () => {
    if (!token) return
    setLoading(true)
    setLoadError(null)
    try {
      const data = await listJobs(
        token,
        queueId,
        stateFilter || undefined,
        50,
      )
      setJobs(data)
    } catch (err: unknown) {
      setLoadError(err instanceof Error ? err.message : 'list failed')
    } finally {
      setLoading(false)
    }
  }, [token, queueId, stateFilter])

  useEffect(() => {
    void load()
  }, [load])

  const runnableDepth = jobs.filter((j) => j.state === 'runnable').length

  return (
    <div>
      <p>
        Queue {queueId} · runnable depth: {runnableDepth}
        {loading ? ' · loading…' : ''}
      </p>

      <div style={{ display: 'flex', gap: '0.75rem', alignItems: 'center', marginBottom: '1rem' }}>
        <label>
          State{' '}
          <select
            value={stateFilter}
            onChange={(e) => setStateFilter(e.target.value)}
          >
            {STATES.map((s) => (
              <option key={s || 'all'} value={s}>
                {s || 'all'}
              </option>
            ))}
          </select>
        </label>
        <button type="button" onClick={() => void load()}>
          Refresh
        </button>
      </div>

      {loadError && (
        <p style={{ color: 'crimson' }}>List error: {loadError}</p>
      )}

      <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.95rem' }}>
        <thead>
          <tr style={{ textAlign: 'left', borderBottom: '1px solid #ccc' }}>
            <th>id</th>
            <th>state</th>
            <th>url</th>
            <th>schedule_id</th>
            <th>attempt_count</th>
          </tr>
        </thead>
        <tbody>
          {jobs.map((job) => (
            <tr
              key={job.id}
              onClick={() => navigate(`/jobs/${job.id}`)}
              style={{
                cursor: 'pointer',
                borderBottom: '1px solid #eee',
              }}
            >
              <td>{job.id}</td>
              <td>{job.state}</td>
              <td style={{ maxWidth: '28rem', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                {job.url}
              </td>
              <td>{job.schedule_id || '—'}</td>
              <td>
                {job.attempt_count}/{job.max_attempts}
              </td>
            </tr>
          ))}
          {!loading && jobs.length === 0 && !loadError && (
            <tr>
              <td colSpan={5}>No jobs</td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  )
}
