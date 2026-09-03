import type { ReactNode } from 'react';
type AuthContextValue = {
    status: 'loading';
    token: null;
    error: null;
} | {
    status: 'error';
    token: null;
    error: string;
} | {
    status: 'ok';
    token: string;
    error: null;
};
interface AuthProviderProps {
    children: ReactNode;
    tenantId?: number;
}
export declare function AuthProvider({ children, tenantId }: AuthProviderProps): import("react").JSX.Element;
export declare function useAuth(): AuthContextValue;
export declare function AuthDebug(): import("react").JSX.Element;
export {};
