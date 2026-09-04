import { useState, useEffect, type FormEvent } from 'react';
import { listCron, enableCron, disableCron, createCron } from './api.js';
import { useAuth } from './AuthProvider.js';
import type { Cron } from './types.js';
import { Link } from 'react-router-dom';


export function CronList(){
    const [crons, setCrons] = useState<Cron[]>([]);
    const [loading, setLoading] =useState(true);
    const [error, setError] = useState('');
    const [refreshKey, setRefreshKey] = useState(0);
    const [cronExpr, setCronExpr] = useState('*/5 * * * *');
    const [url, setUrl] = useState('');
    const [method, setMethod] = useState('GET');
    const [enabled, setEnabled] = useState(false);
    const [submitting, setSubmitting] = useState(false);
    const auth = useAuth();
    const token = auth.status === 'ok' ? auth.token : '';


    async function handleEnable(id: number) {
        await enableCron(token, id);
        setRefreshKey((k) => k + 1);
      }
      async function handleDisable(id: number) {
        await disableCron(token, id);
        setRefreshKey((k) => k + 1);
      }

      async function handleCreate(e: FormEvent) {
        e.preventDefault();
        if (!token) return;
        setSubmitting(true);
        setError('');
        try {
          await createCron(token, {
            queue_id: 1,
            cron_expr: cronExpr,
            timezone: 'UTC',
            url,
            method,
            timeout_ms: 30000,
            max_attempts: 3,
            enabled,
          });
          setUrl('');
          setRefreshKey((k) => k + 1);
        } catch (err) {
          setError(err instanceof Error ? err.message : 'Create failed');
        } finally {
          setSubmitting(false);
        }
      }

    useEffect(()=>{
        if (!token) return;
    
        async function load(){
            setLoading(true);
            setError('');
            try{
                const data = await listCron(token);
                setCrons(data);
            }catch(err){
                setError(err instanceof Error ? err.message: 'Unknown Error');
            }finally{
                setLoading(false);
            }
        }
        void load();
    }, [token, refreshKey]);


    return(
        <div className="Cron-List">

        <h2>Schedules</h2>
        <form onSubmit={(e) => void handleCreate(e)}>
        <p>
            <label>
            Cron expr{' '}
            <input
                type="text"
                required
                value={cronExpr}
                onChange={(e) => setCronExpr(e.target.value)}
            />
            </label>
        </p>
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
            <input
                type="checkbox"
                checked={enabled}
                onChange={(e) => setEnabled(e.target.checked)}
            />{' '}
            Enabled
            </label>
        </p>
        <button type="submit" disabled={submitting || !token}>
            {submitting ? 'Creating…' : 'Create schedule'}
        </button>
        </form>
            {loading && <p>Loading Crons...</p>}
            {error && <p style={{ color: 'red' }}>Error: {error}</p>}
            {!loading && !error && crons.length === 0 && <p>No jobs found</p>}
            {!loading && !error && crons.length > 0 && (
            <ul>
                {crons.map((cron) => (
                    <li key={cron.id}>
                    #{cron.id} — {cron.cron_expr} — {cron.url} — {cron.enabled ? 'enabled' : 'disabled'}
                    {' '}
                    {cron.enabled ? (
                      <button type="button" onClick={() => void handleDisable(cron.id)}>Disable</button>
                    ) : (
                      <button type="button" onClick={() => void handleEnable(cron.id)}>Enable</button>
                    )}
                  </li>
                ))}
            </ul>
            )}
        </div>
    )
}
