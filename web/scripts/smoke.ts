import {
  getToken,
  listJobs,
  getJob,
  replayJob,
  createJob,
  cancelJob,
  listJobAttempts,
  createCron,
  enableCron,
  disableCron,
  listCron,
} from '../src/api.js';

const apiBase = process.env.CHRONOS_API_BASE || 'http://localhost:8080';
console.log(`🚀 Starting smoke test against backend: ${apiBase}`);

async function main(): Promise<void> {
  try {
    // 1. Authenticate
    console.log('\n🔑 Step 1: Fetching token...');
    const { token, expires_at } = await getToken(1);
    console.log(`✅ Token prefix: ${token.substring(0, 20)}... | Expires: ${expires_at}`);

    // 2. List Jobs (limit 10)
    console.log('\n📋 Step 2: Listing top jobs...');
    const jobs = await listJobs({ queue_id: 1, limit: 10 }, token);
    console.log(`✅ Listed ${jobs.length} jobs`);

    // 3. Check Runnable Depth
    console.log('\n📊 Step 3: Checking runnable depth...');
    const runnable = await listJobs({ queue_id: 1, state: 'runnable' }, token);
    console.log(`✅ Runnable depth: ${runnable.length}`);

    // 4. Fetch Single Job Details (If any jobs exist)
    if (jobs.length > 0) {
      console.log('\n🔍 Step 4: Fetching single job details...');
      const firstJob = jobs[0]!; // Destructures safely avoiding strict un-checked index errors
      const job = await getJob(token, firstJob.id);
      console.log(`✅ ID: ${job.id} | State: ${job.state} | URL: ${job.url}`);
    } else {
      console.log('\n⚠️ Step 4 Skipped: No jobs returned in Step 2.');
    }

    // 5. Optional Replay (Only if a dead_lettered job exists)
    console.log('\n🔄 Step 5: Checking for dead_lettered jobs to replay...');
    const deadLetteredJob = jobs.find((j) => j.state === 'dead_lettered');

    if (deadLetteredJob) {
      const replayedJob = await replayJob(token, deadLetteredJob.id);
      console.log(`✅ Replay triggered successfully for ID: ${replayedJob.id}`);
    } else {
      console.log('ℹ️ No dead_lettered jobs found in the batch. Replay step skipped.');
    }

    // 6. Create a fresh job (deterministic path for cancel/attempts)
    console.log('\n🆕 Step 6: Creating job...');
    const key = `smoke-${Date.now()}`;
    const created = await createJob(
      token,
      {
        queue_id: 1,
        url: 'https://example.com',
        method: 'GET',
        max_attempts: 3,
        timeout_ms: 5000,
      },
      key,
    );
    console.log(`✅ Created job #${created.id} (state=${created.state}, key=${key})`);

    // 7. Attempts on a brand-new job should be empty
    console.log('\n📜 Step 7: Listing attempts for created job...');
    const attempts = await listJobAttempts(token, created.id);
    if (attempts.length !== 0) {
      throw new Error(
        `Expected 0 attempts for new job #${created.id}, got ${attempts.length}`,
      );
    }
    console.log(`✅ Attempts length: ${attempts.length}`);

    // 8. Cancel — pass if canceled; skip if worker already claimed/finished
    console.log('\n🛑 Step 8: Canceling created job...');
    try {
      const canceled = await cancelJob(token, created.id);
      if (canceled.state === 'canceled') {
        console.log(`✅ Job #${canceled.id} canceled`);
      } else if (canceled.state === 'running' && canceled.cancel_requested) {
        console.log(
          `ℹ️ Job #${canceled.id} cancel requested while running — worker will ack`,
        );
      } else {
        throw new Error(
          `Unexpected cancel result for #${canceled.id}: state=${canceled.state}`,
        );
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      // Worker may have already moved the job past cancelable states (409)
      if (message.includes('409')) {
        const current = await getJob(token, created.id);
        if (current.state === 'succeeded' || current.state === 'running') {
          console.log(
            `ℹ️ Cancel skipped — job #${created.id} already ${current.state}`,
          );
        } else {
          throw err;
        }
      } else {
        throw err;
      }
    }

    // 9. Create cron (disabled so leader doesn't enqueue during smoke)
      console.log('\n⏰ Step 9: Creating cron...');
      const cron = await createCron(token, {
      queue_id: 1,
      cron_expr: '*/5 * * * *',
      timezone: 'UTC',
      url: 'https://example.com',
      method: 'GET',
      timeout_ms: 5000,
      max_attempts: 3,
      enabled: false,
      });
      console.log(`✅ Created cron #${cron.id} enabled=${cron.enabled}`);

      // 10. Enable
      console.log('\n✅ Step 10: Enabling cron...');
      const enabled = await enableCron(token, cron.id);
      if (!enabled.enabled) throw new Error(`expected enabled=true for #${cron.id}`);
      console.log(`✅ Cron #${enabled.id} enabled`);

      // 11. Disable
      console.log('\n🛑 Step 11: Disabling cron...');
      const disabled = await disableCron(token, cron.id);
      if (disabled.enabled) throw new Error(`expected enabled=false for #${cron.id}`);
      console.log(`✅ Cron #${disabled.id} disabled`);

      // 12. List contains it
      console.log('\n📋 Step 12: Listing cron...');
      const crons = await listCron(token);
      const found = crons.find((c) => c.id === cron.id);
      if (!found) throw new Error(`cron #${cron.id} missing from list`);
      console.log(`✅ Listed ${crons.length} cron(s); found #${cron.id}`);

    console.log('\n🎉 SUCCESS: All steps evaluated safely!');
  } catch (err) {
    console.error('\n❌ Smoke test failed:', err);
    process.exit(1);
  }
}

main();
