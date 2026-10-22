import { describe, expect, it } from "bun:test";

import { browserTelemetryConfig, type BrowserTelemetryEnv } from "./config";

function env(overrides: BrowserTelemetryEnv = {}): BrowserTelemetryEnv {
  return {
    VITE_OTEL_ENABLED: "true",
    VITE_OTEL_EXPORTER_OTLP_ENDPOINT: "http://localhost:4318",
    ...overrides,
  };
}

describe("browserTelemetryConfig", () => {
  it("is disabled by default", () => {
    expect(browserTelemetryConfig({})).toBeNull();
    expect(
      browserTelemetryConfig(env({ VITE_OTEL_ENABLED: "false" })),
    ).toBeNull();
  });

  it("requires an endpoint", () => {
    expect(
      browserTelemetryConfig(env({ VITE_OTEL_EXPORTER_OTLP_ENDPOINT: "  " })),
    ).toBeNull();
  });

  it("normalizes the OTLP traces path", () => {
    expect(browserTelemetryConfig(env())?.traceUrl).toBe(
      "http://localhost:4318/v1/traces",
    );
    expect(
      browserTelemetryConfig(
        env({
          VITE_OTEL_EXPORTER_OTLP_ENDPOINT: "http://collector:4318/",
        }),
      )?.traceUrl,
    ).toBe("http://collector:4318/v1/traces");
    expect(
      browserTelemetryConfig(
        env({
          VITE_OTEL_EXPORTER_OTLP_ENDPOINT: "http://collector:4318/v1/traces",
        }),
      )?.traceUrl,
    ).toBe("http://collector:4318/v1/traces");
  });

  it("applies stable defaults and explicit resource attributes", () => {
    const config = browserTelemetryConfig(
      env({
        VITE_OTEL_SERVICE_NAME: "runcodes-frontend-staging",
        VITE_OTEL_SERVICE_VERSION: "1.2.3",
        VITE_DEPLOYMENT_ENVIRONMENT: "staging",
        VITE_OTEL_TRACES_SAMPLER_ARG: "0.25",
      }),
    );

    expect(config).toMatchObject({
      serviceName: "runcodes-frontend-staging",
      serviceVersion: "1.2.3",
      environment: "staging",
      sampleRatio: 0.25,
    });
  });

  it("clamps invalid sampling ratios and keeps absolute API origins", () => {
    expect(
      browserTelemetryConfig(env({ VITE_OTEL_TRACES_SAMPLER_ARG: "5" }))
        ?.sampleRatio,
    ).toBe(1);
    expect(
      browserTelemetryConfig(env({ VITE_OTEL_TRACES_SAMPLER_ARG: "-1" }))
        ?.sampleRatio,
    ).toBe(0);
    expect(
      browserTelemetryConfig(
        env({
          VITE_OTEL_TRACES_SAMPLER_ARG: "not-a-number",
          VITE_API_ENDPOINT: "http://localhost:8443",
        }),
      ),
    ).toMatchObject({
      sampleRatio: 0.1,
      apiOrigin: "http://localhost:8443",
    });
  });

  it("ignores relative or malformed API endpoints", () => {
    expect(
      browserTelemetryConfig(env({ VITE_API_ENDPOINT: "/api" }))?.apiOrigin,
    ).toBeUndefined();
    expect(
      browserTelemetryConfig(env({ VITE_API_ENDPOINT: "not a url" }))
        ?.apiOrigin,
    ).toBeUndefined();
  });
});
