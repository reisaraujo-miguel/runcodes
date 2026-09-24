/// <reference types="vite/client" />
/// <reference types="vite-plugin-svgr/client" />

// Named environment variables used by the app. Declaring them explicitly keeps
// them typed as `string` instead of falling back to the index signature, even
// when Bun's global types (which type `import.meta.env` as `string | undefined`)
// are loaded for the unit tests.
//
// Both are optional: in the container build Caddy serves the API same-origin, so
// VITE_API_ENDPOINT is unset and the app falls back to the relative /api path.
//
// The contact values are only a fallback for the login page: the platform serves
// them from the database (see the admin panel's "Configurações" page) and the
// build-time value is used until that request answers.
interface ImportMetaEnv {
  readonly VITE_API_ENDPOINT?: string;
  readonly VITE_CONTACT_DISCLAIMER_HTML?: string;
  readonly VITE_CONTACT_EMAIL?: string;
}
