/**
 * Pure configuration for the optional browser telemetry SDK.
 *
 * This module deliberately imports no OpenTelemetry package: it is unit-tested
 * in Bun (which has no DOM) while the browser entry point lives in ./index.ts.
 */

export interface BrowserTelemetryEnv {
  VITE_OTEL_ENABLED?: string;
  VITE_OTEL_EXPORTER_OTLP_ENDPOINT?: string;
  VITE_OTEL_SERVICE_NAME?: string;
  VITE_OTEL_SERVICE_VERSION?: string;
  VITE_DEPLOYMENT_ENVIRONMENT?: string;
  VITE_OTEL_TRACES_SAMPLER_ARG?: string;
  VITE_API_ENDPOINT?: string;
}

export interface BrowserTelemetryConfig {
  /** Full OTLP/HTTP traces URL, ending in /v1/traces. */
  traceUrl: string;
  serviceName: string;
  serviceVersion: string;
  environment: string;
  sampleRatio: number;
  /** Absolute API origin, when the SPA calls a split deployment. */
  apiOrigin?: string;
}

const DEFAULT_SERVICE_NAME = "runcodes-frontend";
const DEFAULT_SERVICE_VERSION = "dev";
const DEFAULT_ENVIRONMENT = "development";
const DEFAULT_SAMPLE_RATIO = 0.1;

function trimmed(value: string | undefined): string {
  return typeof value === "string" ? value.trim() : "";
}

function parseRatio(value: string | undefined): number {
  const raw = trimmed(value);
  if (!raw) {
    return DEFAULT_SAMPLE_RATIO;
  }
  const ratio = Number.parseFloat(raw);
  if (!Number.isFinite(ratio)) {
    return DEFAULT_SAMPLE_RATIO;
  }
  return Math.min(1, Math.max(0, ratio));
}

function tracesUrl(endpoint: string): string {
  const base = endpoint.replace(/\/+$/, "");
  return base.endsWith("/v1/traces") ? base : `${base}/v1/traces`;
}

function absoluteOrigin(value: string | undefined): string | undefined {
  const raw = trimmed(value);
  if (!raw) {
    return undefined;
  }
  try {
    return new URL(raw).origin;
  } catch {
    return undefined;
  }
}

/**
 * Returns null when browser telemetry is disabled or no collector endpoint was
 * configured. Telemetry is opt-in: a build without these variables behaves
 * exactly like the app before instrumentation.
 */
export function browserTelemetryConfig(
  env: BrowserTelemetryEnv,
): BrowserTelemetryConfig | null {
  if (trimmed(env.VITE_OTEL_ENABLED).toLowerCase() !== "true") {
    return null;
  }

  const endpoint = trimmed(env.VITE_OTEL_EXPORTER_OTLP_ENDPOINT);
  if (!endpoint) {
    return null;
  }

  return {
    traceUrl: tracesUrl(endpoint),
    serviceName: trimmed(env.VITE_OTEL_SERVICE_NAME) || DEFAULT_SERVICE_NAME,
    serviceVersion:
      trimmed(env.VITE_OTEL_SERVICE_VERSION) || DEFAULT_SERVICE_VERSION,
    environment:
      trimmed(env.VITE_DEPLOYMENT_ENVIRONMENT) || DEFAULT_ENVIRONMENT,
    sampleRatio: parseRatio(env.VITE_OTEL_TRACES_SAMPLER_ARG),
    apiOrigin: absoluteOrigin(env.VITE_API_ENDPOINT),
  };
}
