import { createRoot } from 'react-dom/client'
import { App as AntdApp } from 'antd'
import App from '@/App'
import { AuthProvider } from '@/features/auth/AuthContext'
import '@/assets/css/global.css'

createRoot(document.getElementById('root')!).render(
  <AntdApp>
    <AuthProvider>
      <App />
    </AuthProvider>
  </AntdApp>
)
