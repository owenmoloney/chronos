import http from 'k6/http';
import {check} from 'k6';

export const options ={
    scenarios: {
        create_jobs: {
            executor: 'constant-arrival-rate',
            rate: 120,
            timeUnit: '1s',
            duration: '60s',
            preAllocatedVUs: 50,
            maxVUs: 200,
        },
    },
    thresholds: {
        http_req_failed: ['rate<0.01'],
        http_reqs: ['rate>100'],
        http_req_duration: ['p(95)<500'],
    },
};
const BASE_URL = __ENV.BASE_URL || 'http://host.docker.internal:8080';

export function setup(){
    const res = http.post(
        `${BASE_URL}/auth/token`,
        JSON.stringify({tenant_id: 1}),
        { headers: { 'Content-Type': 'application/json' } },
    );

    check(res, {'token status 200': (r) => r.status === 200});

    const body = res.json();
    if(!body || !body.token){
        throw new Error(`setup: no token: ${res.status} ${res.body}`);
    }
    return {token: body.token, baseUrl: BASE_URL};
}

export default function (data) {
    const key = `k6-${__VU}-${__ITER}-${Date.now()}`;
    
    const payload = JSON.stringify({
        queue_id: 1,
        url: 'https://example.com',
        method: 'GET',
        timeout_ms: 5000,
        max_attempts: 1,   // fail fast if worker runs it; ingest is what we measure
      });
      const res = http.post(`${data.baseUrl}/jobs`, payload, {
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${data.token}`,
          'Idempotency-Key': key,
        },
      });
      check(res, {
        'create status 201': (r) => r.status === 201,
      });
}
    