import { createRoot } from 'react-dom/client'
import RootProviders from '@/RootProviders'
import '@/assets/css/global.css'

createRoot(document.getElementById('root')!).render(
  <RootProviders />
)
