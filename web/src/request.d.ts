export interface RequestOptions {
    method?: 'GET' | 'POST';
    body?: unknown;
    token?: string;
    headers?: Record<string, string>;
}
export declare function request<T>(path: string, options?: RequestOptions): Promise<T>;
