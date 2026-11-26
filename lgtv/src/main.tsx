import { createRoot } from 'react-dom/client'
import App from './App'
import { installRemote } from './focus'
import './styles.css'

installRemote()
createRoot(document.getElementById('root')!).render(<App />)
