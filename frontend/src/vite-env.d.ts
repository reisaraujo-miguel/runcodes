/// <reference types="vite/client" />
/// <reference types="vite-plugin-svgr/client" />

// Named environment variables used by the app. Declaring them explicitly keeps
// them typed as `string` instead of falling back to the index signature, even
// when Bun's global types (which type `import.meta.env` as `string | undefined`)
// are loaded for the unit tests.
interface ImportMetaEnv {
  readonly VITE_API_ENDPOINT: string;
  readonly VITE_CONTACT_INFO_HTML: string;
}
