import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';

export interface ShellProps{
    children: ReactNode;
}

export function Shell({children}: ShellProps) {
    return(
        <div className="chronos-shell-app" style={{display: 'flex', minHeight: '100vh'}}>

            <aside style={{ width: '200px', background: '#1e1e24', color: '#fff', padding: '1rem'}}>
                <h2>Chronos Ops</h2>
                <nav>
                    <ul style = {{ listStyle: 'none', padding: 0}}>
                        <li style={{margin: '1rem 0' }}>
                            <a href="#" style={{ color: '#fff' }}>Dashboard</a>
                        </li>
                        <li style={{ margin: '1rem 0' }}>
                            <Link to="/" style={{ color: '#fff' }}>Jobs</Link>  
                        </li>
                    </ul>
                </nav>
            </aside>
        <main style= {{flex: 1, padding: '2rem', background: '#f8f9fa'}}>
            { children }
        </main>
        </div>
    );
}
