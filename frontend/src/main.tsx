import React from 'react'
import {createRoot} from 'react-dom/client'
import './style.css'
import App, {ErrorBoundary} from './App'

const container = document.getElementById('root')
if (!container) {
  throw new Error('Root element not found')
}

const root = createRoot(container)

root.render(
    <React.StrictMode>
        <ErrorBoundary>
            <App/>
        </ErrorBoundary>
    </React.StrictMode>
)
