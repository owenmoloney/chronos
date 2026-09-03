import { API_BASE } from './config.js';

export interface RequestOptions{
    method?: 'GET' | 'POST';
    body?: unknown;
    token?: string; 
    headers?: Record<string, string>;
}

export async function request<T>(path: string, options:RequestOptions={}): Promise<T> {
    const { method = 'GET', body, token, headers: extraHeaders } = options;
    const headers: Record<string,string>={
        'Content-Type': 'application/json',
    };
    if (token) {
        headers['Authorization'] = `Bearer ${token}`;
    }
    if (extraHeaders) {
        Object.assign(headers, extraHeaders);
    }
    const fetchOptions: RequestInit = {
        method,
        headers,
    };

    if (body !== undefined) {
        fetchOptions.body = JSON.stringify(body);
    }

    const response = await fetch(`${API_BASE}${path}`, fetchOptions);

    if(!response.ok){
        const text = await response.text();
        throw new Error(`${response.status}: ${text}`);
    }

    const data = (await response.json()) as T;
    return data;


}