import { useState, useEffect } from "react";
import { useAuth } from './AuthProvider.js';
import { listJobs } from './api.js';
import type { Job, JobState } from './types.js';
import { Link } from 'react-router-dom';

export function JobList(){
    const [jobs, setJobs] = useState<Job[]>([]);
    const [loading, setLoading] =useState(true);
    const [error, setError] = useState('');
    const [stateFilter, setStateFilter] = useState<JobState | ''>('');
    const [refreshKey, setRefreshKey] = useState(0);
    const [runnableDepth,setRunnableDepth] = useState<number | null>(null);
    const [depthError, setDepthError] = useState('');
    const [depthLoading, setDepthLoading] = useState(true)
    const QUEUE_ID =1;
    const auth = useAuth();
    const token = auth.status === 'ok' ? auth.token : '';

    useEffect(()=>{
        if (!token) return;
    
        async function load(){
            setLoading(true);
            setError('');
            try{
                const params ={
                    queue_id: QUEUE_ID,
                    limit: 50,
                    ...(stateFilter !== ''? {state: stateFilter}: {}),
                };
            
                const data = await listJobs(params, token);
                setJobs(data);
            }catch(err){
                setError(err instanceof Error ? err.message: 'Unknown Error');
            }finally{
                setLoading(false);
            }
        }
        void load();
    }, [token, stateFilter, refreshKey]);

    useEffect(()=>{
        if(!token) return;

        async function loadDepth(){
            setDepthLoading(true);
            setDepthError('');
            try{
                const runnable = await listJobs(
                    {queue_id: QUEUE_ID, state: 'runnable', limit: 200},
                    token,
                );
                setRunnableDepth(runnable.length);
            }catch(err){
                setDepthError(err instanceof Error ? err.message: 'Unknown Error');
            }finally{
                setDepthLoading(false)
            }
        }
        void loadDepth();
    }, [token, refreshKey]);

    const JOB_STATES: JobState[] = [
        'pending', 'runnable', 'running', 'succeeded',
        'failed_retrying', 'dead_lettered', 'canceled',
      ];

    return(
        <div className="Job-List">
            <p style={{ fontWeight: runnableDepth && runnableDepth > 0 ? 600 : 400 }}>
            Runnable: {depthLoading ? '…' : runnableDepth ?? '…'}</p>
            {depthError && <p style={{ color: 'red' }}>Depth error: {depthError}</p>}
            {loading && <p>Loading jobs...</p>}
            {error && <p style={{ color: 'red' }}>Error: {error}</p>}
            {!loading && !error && jobs.length === 0 && <p>No jobs found</p>}
            {!loading && !error && jobs.length > 0 && (
            <ul>
                {jobs.map((job) => (
                    <li key={job.id}>
                        <Link to={`/jobs/${job.id}`}>
                            #{job.id} — {job.state} — {job.url}
                        </Link>
                    </li>
                ))}
            </ul>
            )}
            <Link to="/jobs/new">Create job</Link>
            <button type="button" disabled={loading} onClick={() => setRefreshKey((k) => k + 1)}>
            Refresh
            </button>
            <select value={stateFilter} onChange={(e) => setStateFilter(e.target.value as JobState | '')}>
                <option value="">All</option>
                {JOB_STATES.map((s) => (
                    <option key={s} value={s}>{s}</option>
                ))}
            </select>
        </div>
    )
}

