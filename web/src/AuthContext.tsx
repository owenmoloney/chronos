import {createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { getToken } from './api'


type AuthValue = {
    token: string | null
    error: string | null
    queueId: number
}

const AuthContext = createContext<AuthValue | null>(null)


export function AuthProvider({children}:{children: ReactNode}){
    const [token, setToken] = useState<string | null>(null)
    const [error, setError] = useState<string | null>(null)
    const queueId = 1
    useEffect(() => {
        let cancelled = false
      
        getToken(1)
          .then((t) => {
            if (!cancelled) {
              setToken(t)
              setError(null)
            }
          })
          .catch((err: unknown) => {
            if (!cancelled) {
              const msg = err instanceof Error ? err.message : 'failed to get token'
              setError(msg)
              setToken(null)
            }
          })
      
        return () => {
          cancelled = true
        }
      }, [])
      
    return (
        <AuthContext.Provider value={{ token, error, queueId }}>
        {children}
        </AuthContext.Provider>
    )
}

export function useAuth(): AuthValue {
    const ctx = useContext(AuthContext)
    if (ctx === null) {
      throw new Error('useAuth must be used inside AuthProvider')
    }
    return ctx
  }

