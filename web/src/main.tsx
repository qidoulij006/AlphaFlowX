import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App.tsx'
import { Toaster } from 'sonner'
import './index.css'
import { BrowserRouter } from 'react-router-dom'
import { ThemeProvider, useTheme } from './contexts/ThemeContext'

function ThemedToaster() {
  const { resolvedTheme } = useTheme()

  return (
    <Toaster
      theme={resolvedTheme}
      richColors
      closeButton
      position="top-center"
      duration={2200}
      toastOptions={{
        className: 'nofx-toast',
        style: {
          background: 'var(--panel-bg-solid)',
          border: '1px solid var(--panel-border)',
          color: 'var(--text-primary)',
        },
      }}
    />
  )
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ThemeProvider>
      <BrowserRouter>
        <ThemedToaster />
        <App />
      </BrowserRouter>
    </ThemeProvider>
  </React.StrictMode>
)
