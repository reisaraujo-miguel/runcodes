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

  // Browser telemetry (see src/lib/telemetry). All are optional; tracing is
  // disabled unless VITE_OTEL_ENABLED is "true" and an endpoint is set.
  readonly VITE_OTEL_ENABLED?: string;
  readonly VITE_OTEL_EXPORTER_OTLP_ENDPOINT?: string;
  readonly VITE_OTEL_SERVICE_NAME?: string;
  readonly VITE_OTEL_SERVICE_VERSION?: string;
  readonly VITE_DEPLOYMENT_ENVIRONMENT?: string;
  readonly VITE_OTEL_TRACES_SAMPLER_ARG?: string;
}
