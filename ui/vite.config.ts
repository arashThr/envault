import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: '../cmd/envault-server/web',
    emptyOutDir: true,
  },
})
