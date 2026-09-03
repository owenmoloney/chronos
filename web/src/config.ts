function readApiBase(): string {
    const env = (globalThis as {
      process?: { env?: Record<string, string | undefined> };
    }).process?.env;
  
    return env?.CHRONOS_API_BASE ?? '/api';
  }
  
  export const API_BASE = readApiBase();