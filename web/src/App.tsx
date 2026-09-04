import { Shell } from "./Shell.js";
import { AuthProvider} from "./AuthProvider.js";
import { JobList } from './JobList.js';
import {Routes, Route} from 'react-router-dom';
import { JobDetail } from './JobDetail.js';
import {JobCreate} from './JobCreate.js';
import {CronList} from './CronList.js'

function App(){
    return (
        <Shell> 
            <AuthProvider>
                <Routes>
                    <Route path="/jobs/new" element={<JobCreate />} />
                    <Route path="/jobs/:id" element={<JobDetail />} />
                    <Route path="/" element = {<JobList/>} />
                    <Route path="/cron" element={<CronList />}/>
                </Routes>
            </AuthProvider>
        </Shell>
    )
}

export default App;