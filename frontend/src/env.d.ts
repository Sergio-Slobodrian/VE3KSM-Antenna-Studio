// Copyright 2026 Sergio Slobodrian
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
