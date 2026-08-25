import { useCallback, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { getJob, replayJob, type Job } from './api'
import { useAuth } from './AuthContext'

export function JobDetail() {
  const { id } = useParams<{ id: string }>()
  const { token } = useAuth()
  const [job, setJob] = useState<Job | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [replaying, setReplaying] = useState(false)

  const load = useCallback(async () => {
    if (!token || !id) return
    setLoading(true)
    setError(null)
    try {
      const data = await getJob(token, id)
      setJob(data)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'failed to load job')
      setJob(null)
    } finally {
      setLoading(false)
    }
  }, [token, id])

  useEffect(() => {
    void load()
  }, [load])

  async function onReplay() {
    if (!token || !id) return
    setReplaying(true)
    setError(null)
    try {
      const updated = await replayJob(token, id)
      setJob(updated)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'replay failed')
    } finally {
      setReplaying(false)
    }
  }

  if (!id) {
    return <p style={{ color: 'crimson' }}>Missing job id</p>
  }

  return (
    <div>
      <p>
        <Link to="/">← Jobs</Link>
      </p>

      <h2 style={{ marginTop: 0 }}>Job {id}</h2>

      {loading && !job && <p>Loading…</p>}
      {error && <p style={{ color: 'crimson' }}>{error}</p>}

      {job && (
        <>
          <dl
            style={{
              display: 'grid',
              gridTemplateColumns: '10rem 1fr',
              gap: '0.35rem 1rem',
              margin: '1rem 0',
            }}
          >
            <dt>state</dt>
            <dd style={{ margin: 0 }}>{job.state}</dd>
            <dt>url</dt>
            <dd style={{ margin: 0 }}>{job.url}</dd>
            <dt>method</dt>
            <dd style={{ margin: 0 }}>{job.method}</dd>
            <dt>schedule_id</dt>
            <dd style={{ margin: 0 }}>{job.schedule_id || '—'}</dd>
            <dt>attempts</dt>
            <dd style={{ margin: 0 }}>
              {job.attempt_count}/{job.max_attempts}
            </dd>
            <dt>locked_by</dt>
            <dd style={{ margin: 0 }}>{job.locked_by || '—'}</dd>
            <dt>locked_at</dt>
            <dd style={{ margin: 0 }}>{job.locked_at || '—'}</dd>
            <dt>run_at</dt>
            <dd style={{ margin: 0 }}>{job.run_at || '—'}</dd>
            <dt>next_run_at</dt>
            <dd style={{ margin: 0 }}>{job.next_run_at || '—'}</dd>
            <dt>cancel_requested</dt>
            <dd style={{ margin: 0 }}>{String(!!job.cancel_requested)}</dd>
            <dt>updated_at</dt>
            <dd style={{ margin: 0 }}>{job.updated_at}</dd>
          </dl>

          {job.state === 'dead_lettered' && (
            <button
              type="button"
              onClick={() => void onReplay()}
              disabled={replaying}
            >
              {replaying ? 'Replaying…' : 'Replay'}
            </button>
          )}
        </>
      )}
    </div>
  )
}
