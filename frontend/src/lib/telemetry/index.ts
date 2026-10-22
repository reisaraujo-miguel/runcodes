import { W3CTraceContextPropagator } from "@opentelemetry/core";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-http";
import { registerInstrumentations } from "@opentelemetry/instrumentation";
import { FetchInstrumentation } from "@opentelemetry/instrumentation-fetch";
import { resourceFromAttributes } from "@opentelemetry/resources";
import {
  BatchSpanProcessor,
  ParentBasedSampler,
  StackContextManager,
  TraceIdRatioBasedSampler,
  WebTracerProvider,
} from "@opentelemetry/sdk-trace-web";
import {
  ATTR_DEPLOYMENT_ENVIRONMENT_NAME,
  ATTR_SERVICE_NAME,
  ATTR_SERVICE_VERSION,
} from "@opentelemetry/semantic-conventions";

import { browserTelemetryConfig } from "./config";

/**
 * Initializes browser traces for the SPA.
 *
 * The app uses `fetch` for every API call, so `FetchInstrumentation` both
 * creates the client span and injects the W3C `traceparent` header. The backend
 * extracts it in its OTel middleware, which is what turns a click into a trace
 * spanning browser -> Caddy -> backend (and the judge, for submissions).
 *
 * This is intentionally fire-and-forget: telemetry must never prevent the app
 * from rendering, and a collector that is down only drops spans.
 */
export function initBrowserTelemetry(): void {
  if (typeof window === "undefined") {
    return;
  }

  const config = browserTelemetryConfig(import.meta.env);
  if (!config) {
    return;
  }

  try {
    const exporter = new OTLPTraceExporter({ url: config.traceUrl });

    const provider = new WebTracerProvider({
      resource: resourceFromAttributes({
        [ATTR_SERVICE_NAME]: config.serviceName,
        [ATTR_SERVICE_VERSION]: config.serviceVersion,
        [ATTR_DEPLOYMENT_ENVIRONMENT_NAME]: config.environment,
      }),
      sampler: new ParentBasedSampler({
        root: new TraceIdRatioBasedSampler(config.sampleRatio),
      }),
      spanProcessors: [
        new BatchSpanProcessor(exporter, {
          scheduledDelayMillis: 2_000,
          maxQueueSize: 512,
          maxExportBatchSize: 64,
          exportTimeoutMillis: 10_000,
        }),
      ],
    });

    provider.register({
      contextManager: new StackContextManager(),
      propagator: new W3CTraceContextPropagator(),
    });

    const propagateTo = [
      new RegExp(`^${escapeRegExp(window.location.origin)}`),
    ];
    if (config.apiOrigin) {
      propagateTo.push(new RegExp(`^${escapeRegExp(config.apiOrigin)}`));
    }

    registerInstrumentations({
      tracerProvider: provider,
      instrumentations: [
        new FetchInstrumentation({
          // Send traceparent only to our own API, not to every third-party URL.
          propagateTraceHeaderCorsUrls: propagateTo,
          // Never trace the OTLP export request itself.
          ignoreUrls: [/\/v1\/traces(?:\?|$)/],
          applyCustomAttributesOnSpan: (span) => {
            // The page an API call came from is useful when one screen makes
            // calls to several endpoints but only one of them is slow.
            span.setAttribute("browser.url.path", window.location.pathname);
          },
        }),
      ],
    });
  } catch (error) {
    // The console is the only fallback when telemetry setup itself fails; the
    // application must still boot. Never include the endpoint credentials in
    // this message (there are none today).
    console.warn("OpenTelemetry browser tracing disabled:", error);
  }
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}
