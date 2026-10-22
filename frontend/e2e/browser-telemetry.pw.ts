import { expect, test } from "@playwright/test";

const tempoURL = process.env.TEMPO_URL ?? "http://127.0.0.1:3200";

interface TraceValue {
  stringValue?: string;
  intValue?: string;
}

interface SpanAttribute {
  key: string;
  value: TraceValue;
}

interface Span {
  name: string;
  attributes?: SpanAttribute[];
}

interface Batch {
  resource?: { attributes?: SpanAttribute[] };
  scopeSpans?: { spans?: Span[] }[];
}

interface Trace {
  batches?: Batch[];
}

function serviceNames(trace: Trace): string[] {
  const names: string[] = [];
  for (const batch of trace.batches ?? []) {
    for (const attribute of batch.resource?.attributes ?? []) {
      if (attribute.key === "service.name" && attribute.value.stringValue) {
        names.push(attribute.value.stringValue);
      }
    }
  }
  return [...new Set(names)];
}

function spans(trace: Trace): Span[] {
  return (trace.batches ?? []).flatMap((batch) =>
    (batch.scopeSpans ?? []).flatMap((scope) => scope.spans ?? []),
  );
}

test("browser fetch spans continue into the backend and reach Tempo", async ({
  page,
}) => {
  await page.goto("/");
  // The SPA awaits the lazy telemetry import before mounting React. Give the
  // module a moment to install FetchInstrumentation before issuing the probe.
  await page.waitForTimeout(1_000);

  const marker = `browser-smoke-${String(Date.now())}`;
  const requestPromise = page.waitForRequest(
    (request) => request.url().includes(marker),
    { timeout: 15_000 },
  );

  const status = await page.evaluate(async (probeMarker) => {
    const response = await fetch(`/api/v1/settings/public?${probeMarker}`, {
      cache: "no-store",
    });
    return response.status;
  }, marker);

  expect(status).toBe(200);

  const request = await requestPromise;
  const traceparent = request.headers().traceparent;
  if (!traceparent) {
    throw new Error(
      "no traceparent on the browser request: build the SPA with VITE_OTEL_ENABLED=true and a browser-reachable collector URL",
    );
  }
  expect(traceparent).toMatch(
    /^[0-9a-f]{2}-[0-9a-f]{32}-[0-9a-f]{16}-[0-9a-f]{2}$/,
  );

  const traceId = traceparent.split("-")[1];
  if (!traceId) {
    throw new Error(`malformed traceparent: ${traceparent}`);
  }

  await expect
    .poll(
      async () => {
        const response = await fetch(`${tempoURL}/api/traces/${traceId}`);
        if (!response.ok) return null;
        const trace = (await response.json()) as Trace;
        const services = serviceNames(trace);
        const sawBrowserProbe = spans(trace).some((span) =>
          span.attributes?.some(
            (attribute) =>
              (attribute.key === "url.full" || attribute.key === "http.url") &&
              attribute.value.stringValue?.includes(marker),
          ),
        );
        return {
          frontend: services.includes("runcodes-frontend"),
          backend: services.includes("runcodes-backend"),
          sawBrowserProbe,
        };
      },
      { timeout: 60_000, intervals: [1_000, 2_000, 3_000] },
    )
    .toEqual({ frontend: true, backend: true, sawBrowserProbe: true });

  const response = await fetch(`${tempoURL}/api/traces/${traceId}`);
  expect(response.ok).toBe(true);
  const trace = (await response.json()) as Trace;
  console.log(
    `browser trace ${traceId} contains ${String(spans(trace).length)} spans`,
  );
});
