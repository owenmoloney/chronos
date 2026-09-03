import { useState, type FormEvent } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { useAuth } from './AuthProvider.js';
import { createJob } from './api.js';

const QUEUE_ID = 1;

export function JobCreate() {
  const auth = useAuth();
  const navigate = useNavigate();
  const token = auth.status === 'ok' ? auth.token : '';

  const [url, setUrl] = useState('');
  const [method, setMethod] = useState('POST');
  const [maxAttempts, setMaxAttempts] = useState(3);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');
  const [idempotencyKey, setIdempotencyKey] = useState('');



  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!token) return;

    setSubmitting(true);
    setError('');
    try {
      const created = await createJob(
        token,
        {
        queue_id: QUEUE_ID,
        url,
        method,
        max_attempts: maxAttempts,
        timeout_ms: 30000,
        },
        idempotencyKey,
      );
      navigate(`/jobs/${created.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Create failed');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div>
      <Link to="/">← Back to jobs</Link>
      <h2>Create job</h2>
      <form onSubmit={(e) => void handleSubmit(e)}>
        <p>
          <label>
            URL{' '}
            <input
              type="url"
              required
              value={url}
              onChange={(e) => setUrl(e.target.value)}
            />
          </label>
        </p>
        <p>
          <label>
            Method{' '}
            <select value={method} onChange={(e) => setMethod(e.target.value)}>
              <option value="GET">GET</option>
              <option value="POST">POST</option>
              <option value="PUT">PUT</option>
              <option value="DELETE">DELETE</option>
            </select>
          </label>
        </p>
        <p>
          <label>
            Max attempts{' '}
            <input
              type="number"
              min={1}
              value={maxAttempts}
              onChange={(e) => setMaxAttempts(Number(e.target.value))}
            />
          </label>
        </p>
        <p>
            <label>
                Idempotency key (optional){' '}
                <input
                  type="text"
                  value = {idempotencyKey}
                  onChange={(e) => setIdempotencyKey(e.target.value)}
                  placeholder='e.g. import-batch-42'
                />
            </label>
        </p>
        <button type="submit" disabled={submitting}>
          {submitting ? 'Creating...' : 'Create job'}
        </button>
    </form>
      {error && <p style={{ color: 'red' }}>{error}</p>}
    </div>
  );
}