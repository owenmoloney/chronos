import { createContext, useContext, useEffect, useState } from 'react';
import type { ReactNode } from 'react';
import { getToken } from './api.js';



type AuthContextValue =
    |{status: 'loading'; token: null; error: null}
    |{status: 'error'; token: null; error: string}
    |{status: 'ok'; token: string; error: null}


interface AuthProviderProps {
    children: ReactNode;
    tenantId?: number;
}


const AuthContext = createContext<AuthContextValue | null>(null);


export function AuthProvider({ children, tenantId = 1 }: AuthProviderProps){
    const [status, setStatus] = useState< 'loading' | 'ok' | 'error'>('loading');
    const [token, setToken] = useState<string>('');
    const [error, setError] = useState<string>('');

    useEffect(() => {
        async function load(){
            try{
                const {token} = await getToken(tenantId);
                await new Promise((resolve) => setTimeout(resolve, 2000)); 
                setToken(token);
                setStatus('ok');
            } catch(err){
                setError(err instanceof Error ? err.message : 'Unknown Error');
                setStatus('error');
            }
        }
        void load();
    }, [tenantId]);

    if (status === 'loading'){
        return <p>Authenticating...</p>;
    }

    if (status === 'error'){
        return <p> Error: {error}</p>
    }
    const value: AuthContextValue = {
        status: 'ok',
        token,
        error: null,
      };
      return (
        <AuthContext.Provider value={value}>
          {children}
        </AuthContext.Provider>
      );
}
export function useAuth():  AuthContextValue{
    const ctx = useContext(AuthContext);
    if (ctx === null){
        throw new Error('useAuth must be used within AuthProvider');
    }
    return ctx;
}

export function AuthDebug() {
    const { token } = useAuth();
    return <p>Token: {token?.slice(0, 20)}...</p>;
  }