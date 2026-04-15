/**
 * Ambient type declarations for the build-time environment.
 *
 * The Go backend (backend/internal/assets) bundles this TypeScript tree
 * via esbuild and injects a replacement for `import.meta.env.VITE_API_BASE`
 * at build time.  This file exists purely so editors and `tsc --noEmit`
 * type-check cleanly; nothing here reaches the browser.
 */
interface ImportMetaEnv {
  readonly VITE_API_BASE?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
