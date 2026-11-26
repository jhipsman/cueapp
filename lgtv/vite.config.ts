import { defineConfig, type Plugin } from 'vite'
import react from '@vitejs/plugin-react'

// The LG app's pages are files on the TV (file://), opened by the TV's web
// engine, which on 2018-2019 sets is an older Chromium (53): one plain
// script (no ES modules, no "crossorigin", which a file page can't load),
// its newer syntax rewritten for that engine, relative paths.
function classicScript(): Plugin {
  return {
    name: 'cue-classic-script',
    enforce: 'post',
    transformIndexHtml(html) {
      return html
        .replace(/<script type="module" crossorigin src="([^"]+)"><\/script>/g, '')
        .replace(/ crossorigin/g, '')
        .replace('</body>', '<script src="./app.js"></script>\n</body>')
    },
  }
}

export default defineConfig({
  base: './',
  plugins: [react(), classicScript()],
  build: {
    outDir: 'dist-app',
    target: ['chrome53'],
    modulePreload: false,
    cssCodeSplit: false,
    rollupOptions: {
      output: {
        format: 'iife',
        inlineDynamicImports: true,
        entryFileNames: 'app.js',
        assetFileNames: '[name][extname]',
      },
    },
  },
})
