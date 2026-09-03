import { useState, useEffect} from 'react';
import { Link, useParams, useNavigate } from 'react-router-dom';
import type { Job } from './types.js';
import { useAuth } from './AuthProvider.js';
import { getJob, replayJob, cancelJob, listJobAttempts } from './api.js';
import { type JobAttempt } from './types.js';



export function JobDetail(){
    const [job, setJob] = useState<Job | null>(null);
    const [successMessage, setSuccessMessage] = useState('')
    const [loading, setLoading] =useState(true);
    const [error, setError] = useState('');
    const [replaying,setReplaying] = useState(false)
    const [replayError, setReplayError] = useState('');
    const [canceling, setCanceling] = useState(false);
    const [cancelError, setCancelError] = useState('');
    const [attempts, setAttempts] = useState<JobAttempt[]>([]);
    const [attemptsLoading, setAttemptsLoading] = useState(true);
    const [attemptsError, setAttemptsError] = useState('');
    const params = useParams();
    const auth = useAuth();
    const navigate = useNavigate();
    const id = Number(params.id);
    const validId = Number.isFinite(id) ? id : null;
    const token = auth.status === 'ok' ? auth.token : '';
    const canCancel =
      !job?.cancel_requested &&
      (job?.state === 'pending' || job?.state === 'runnable' || job?.state === 'running');

    useEffect(() => {
        if (!token || validId === null) return;
        async function load(){
            setLoading(true);
            setError('');
            try {
                const data = await getJob(token, validId!);
                setJob(data);
              } catch (err) {
                setError(err instanceof Error ? err.message : 'Unknown error');
              } finally {
                setLoading(false);
              }
            }
            void load();
          }, [token, validId]);
      
    useEffect(() => {
      if (!token || validId === null) return;
    
      async function loadAttempts() {
        setAttemptsLoading(true);
        setAttemptsError('');
        try {
          const data = await listJobAttempts(token, validId!);
          setAttempts(data);
        } catch (err) {
          setAttemptsError(err instanceof Error ? err.message : 'Unknown error');
        } finally {
          setAttemptsLoading(false);
        }
      }
      void loadAttempts();
    }, [token, validId]);
          
    if (validId === null) return <p>Invalid job id</p>;
    
    async function handleReplay() {
      if(!token || validId  === null || job === null) return;

      setReplaying(true);
      setReplayError('');
      try{
        const updated = await replayJob(token, validId);
        setJob(updated);
        setSuccessMessage('Replayed — job is runnable again');
        navigate('/');
      } catch(err){
        setReplayError(err instanceof Error? err.message: 'Replay Failed');
      } finally{
        setReplaying(false);
      }
    }

    async function handleCancel() {
      if(!token || validId  === null || job === null) return;

      setCanceling(true);
      setCancelError('');
      setSuccessMessage('');
      try{
        const updated = await cancelJob(token, validId);
        setJob(updated);
        if(updated.state === 'canceled'){
          setSuccessMessage('Job canceled');
        } else if(updated.cancel_requested === true){
          setSuccessMessage('Cancel requested — worker will stop job');
        }
      } catch(err){
        setCancelError(err instanceof Error? err.message: 'Cancel failed');
      } finally{
        setCanceling(false);
      }
    }

    return (
      <div>
        <Link to="/">← Back to jobs</Link>
          {loading && <p>Loading job...</p>}
          {error && <p style={{ color: 'red' }}>Error: {error}</p>}
          {job && (
            <>
              <p><strong>ID:</strong> {job.id}</p>
              <p><strong>Tenant ID:</strong> {job.tenant_id}</p>
              <p><strong>Queue ID:</strong> {job.queue_id}</p>
              <p><strong>Schedule ID:</strong> {job.schedule_id || '—'}</p>
              <p><strong>State:</strong> {job.state}</p>
              <p><strong>Method:</strong> {job.method}</p>
              <p><strong>URL:</strong> {job.url}</p>
              <p><strong>timeout_ms:</strong> {job.timeout_ms}</p>
              <p><strong>Header:</strong></p>
                  <pre>{JSON.stringify(job.headers, null, 2)}</pre>
              <p><strong>Body:</strong></p> 
                  <pre>{JSON.stringify(job.body, null, 2)}</pre>
              <p><strong>run_at:</strong> {job.run_at?.startsWith('0001-01-01') ? '—' : (job.run_at || '—')}</p>
              <p><strong>Next run at:</strong> {job.next_run_at?.startsWith('0001-01-01') ? '—' : (job.next_run_at || '—')}</p>
              <p><strong>Attempts:</strong> {job.attempt_count} / {job.max_attempts}</p>
              <p><strong>cancel_requested:</strong> {job.cancel_requested? 'Yes':'No'}</p>
              <p><strong>Locked By:</strong> {job.locked_by || '—'}</p>
              <p><strong>Locked At:</strong> {job.locked_at?.startsWith('0001-01-01') ? '—' :(job.locked_at) || '—'}</p>
              <p><strong>Idempotency Key:</strong> {job.idempotency_key || '—'}</p>
              <p><strong>Created:</strong> {job.created_at}</p>
              <p><strong>Updated:</strong> {job.updated_at}</p>
            </>
          )}
          {job?.state === 'dead_lettered' && (
            <button 
              type="button" 
              onClick={()=>void handleReplay()}
              disabled ={replaying}
              >
              {replaying ? 'Replaying...' : 'Replay'}
            </button>
          )}
          {canCancel && (
            <button
              type="button"
              onClick={() => void handleCancel()}
              disabled={canceling}
            >
              {canceling ? 'Canceling...' : 'Cancel job'}
          </button>
          )}
          {cancelError && <p style={{ color: 'red' }}>Cancel error: {cancelError}</p>}
          {successMessage && <p style={{ color: 'green', fontWeight: 'bold' }}>{successMessage}</p>}
          {replayError && (
            <p style={{ color: 'red' }}>Replay error: {replayError}</p>
          )}
          <h3>Attempt history</h3>
          {attemptsLoading && <p>Loading attempts...</p>}
          {attemptsError && <p style={{ color: 'red' }}>{attemptsError}</p>}
          {!attemptsLoading && !attemptsError && attempts.length === 0 && (
            <p>No attempts yet</p>
          )}
          {!attemptsLoading && attempts.length > 0 && (
            <ul>
              {attempts.map((a) => (
                <li key={a.attempt_number}>
                  #{a.attempt_number} — {a.success === 'true' ? 'succeeded' : 'failed'}
                  {' '}— HTTP {a.http_status || '—'} — {a.error_message || '—'}
                  {a.response_snippet && (
                    <pre>{a.response_snippet.slice(0, 200)}</pre>
                  )}
                </li>
              ))}
            </ul>
          )}
      </div>
    );
}