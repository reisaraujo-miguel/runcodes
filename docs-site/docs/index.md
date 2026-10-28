---
layout: home

hero:
  name: RunCodes
  text: Run programming exercises at scale
  tagline: An open-source platform for programming courses. Students submit code, isolated containers compile and run it against test cases, and results stream back live.
  image:
    src: /runcodes-icon.svg
    alt: RunCodes
  actions:
    - theme: brand
      text: Get started
      link: /guide/getting-started
    - theme: alt
      text: Explore the architecture
      link: /guide/architecture

features: []
---

<div class="rc-facts">
  <div class="rc-fact">
    <strong>22</strong>
    <span>language runner images, from C and Java to Prolog and Verilog</span>
  </div>
  <div class="rc-fact">
    <strong>Live</strong>
    <span>per-test-case verdicts over Server-Sent Events</span>
  </div>
  <div class="rc-fact">
    <strong>Rootless</strong>
    <span>every graded run executes in an isolated Podman container</span>
  </div>
  <div class="rc-fact">
    <strong>4 hosts</strong>
    <span>reference Ansible deployment with full observability</span>
  </div>
</div>

<div class="rc-home-section">
  <p class="rc-eyebrow">Documentation</p>
  <h2 class="rc-section-title">Everything from first login to on-call</h2>
  <p class="rc-section-lede">
    The documentation covers the product, every service in the stack, the HTTP
    API, both deployment paths, and the observability stack that ties them
    together.
  </p>

  <div class="rc-grid">
    <a class="rc-card" href="./guide/architecture.html">
      <span class="rc-card-icon">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="3" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="14" width="7" height="7" rx="1.5"/><path d="M10 6.5h4a3.5 3.5 0 0 1 3.5 3.5v4"/><path d="M6.5 10v3.5A3.5 3.5 0 0 0 10 17h4"/></svg>
      </span>
      <h3>Architecture</h3>
      <p>How the frontend, backend, judge, database and object store fit together, and how a submission moves through them.</p>
      <span class="rc-arrow">Read the architecture →</span>
    </a>
    <a class="rc-card" href="./guide/getting-started.html">
      <span class="rc-card-icon">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="3" y="4" width="18" height="16" rx="2.5"/><path d="m7 9 3 3-3 3"/><path d="M13 15h4"/></svg>
      </span>
      <h3>Quick start</h3>
      <p>Bring the whole stack up with Docker Compose, create the first admin, and verify an account.</p>
      <span class="rc-arrow">Start locally →</span>
    </a>
    <a class="rc-card" href="./guide/submissions.html">
      <span class="rc-card-icon">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M13 2 4.5 13.5H11L10 22l8.5-11.5H12L13 2Z"/></svg>
      </span>
      <h3>Submissions & judging</h3>
      <p>The durable queue, the judge state machine, per-case verdicts and the SSE event stream.</p>
      <span class="rc-arrow">Follow a submission →</span>
    </a>
    <a class="rc-card" href="./api/">
      <span class="rc-card-icon">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M8 4c-2 0-3 1-3 3v3c0 1.7-1 2.5-2 2 1-.5 2 .3 2 2v3c0 2 1 3 3 3"/><path d="M16 4c2 0 3 1 3 3v3c0 1.7 1 2.5 2 2-1-.5-2 .3-2 2v3c0 2-1 3-3 3"/></svg>
      </span>
      <h3>API reference</h3>
      <p>Authentication, classes, exercises, submissions, administration and the internal judge contract.</p>
      <span class="rc-arrow">Browse endpoints →</span>
    </a>
    <a class="rc-card" href="./deployment/">
      <span class="rc-card-icon">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="3" y="3" width="18" height="7" rx="1.8"/><rect x="3" y="14" width="18" height="7" rx="1.8"/><path d="M7 6.5h.01M7 17.5h.01"/></svg>
      </span>
      <h3>Deployment</h3>
      <p>Run it on one host with Compose, or on four machines with Ansible, rootless Podman and GHCR images.</p>
      <span class="rc-arrow">Choose a deployment →</span>
    </a>
    <a class="rc-card" href="./observability/">
      <span class="rc-card-icon">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3 20h18"/><path d="M6 16v-5"/><path d="M11 16V7"/><path d="M16 16v-8"/><path d="M21 16V4"/></svg>
      </span>
      <h3>Observability</h3>
      <p>Traces, metrics, logs, CPU profiles and alerts across the Go services and the browser.</p>
      <span class="rc-arrow">Open the runbook →</span>
    </a>
  </div>
</div>

<div class="rc-home-section" style="margin-top: 72px;">
  <p class="rc-eyebrow">Choose your path</p>
  <h2 class="rc-section-title">Documentation for every role</h2>
  <p class="rc-section-lede">
    The same platform looks different from a student desk, a professor's
    gradebook and an administrator's console. Start with the guide for your
    role.
  </p>

  <div class="rc-grid">
    <a class="rc-card" href="./guide/student-guide.html">
      <span class="rc-card-icon">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 5.5A2.5 2.5 0 0 1 6.5 3H20v16H6.5A2.5 2.5 0 0 0 4 21.5Z"/><path d="M4 5.5v16"/><path d="M8 7h8"/><path d="M8 11h6"/></svg>
      </span>
      <h3>Student guide</h3>
      <p>Create an account, join a class, submit solutions, read verdicts and download your work.</p>
      <span class="rc-arrow">Start as a student →</span>
    </a>
    <a class="rc-card" href="./guide/professor-guide.html">
      <span class="rc-card-icon">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3 7.5 12 3l9 4.5-9 4.5Z"/><path d="M7 10v4.5c0 1.5 2.2 2.5 5 2.5s5-1 5-2.5V10"/><path d="M21 7.5V14"/></svg>
      </span>
      <h3>Professor guide</h3>
      <p>Create classes, author exercises and test cases, grade a class and reuse the material.</p>
      <span class="rc-arrow">Teach with RunCodes →</span>
    </a>
    <a class="rc-card" href="./administration/">
      <span class="rc-card-icon">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 3 4.5 6v5.5c0 4.4 3 7.6 7.5 9.5 4.5-1.9 7.5-5.1 7.5-9.5V6Z"/><path d="m9 12 2 2 4-4"/></svg>
      </span>
      <h3>Administrator guide</h3>
      <p>Manage accounts and classes, edit settings and file types, and operate the deployment.</p>
      <span class="rc-arrow">Administer the platform →</span>
    </a>
  </div>
</div>

<div class="rc-home-section" style="margin-top: 72px;">
  <p class="rc-eyebrow">How it works</p>
  <h2 class="rc-section-title">A durable pipeline, from upload to verdict</h2>
  <p class="rc-section-lede">
    PostgreSQL is the queue. The backend owns everything that must survive a
    restart; the judge owns everything that touches untrusted code. A submission
    is never silently lost.
  </p>

  <div class="rc-arch">
    <div>
      <h3>Separation of trust and persistence</h3>
      <p>
        The API registers a <code>queued</code> commit, uploads the source to
        SeaweedFS and wakes the judge. The judge claims the commit with
        <code>FOR UPDATE SKIP LOCKED</code>, runs the matching language image
        under rootless Podman, and streams results back over SSE.
      </p>
      <ul>
        <li>The judge writes only the claim — never results or artifacts.</li>
        <li>Graded containers have no network namespace and no new privileges.</li>
        <li>The backend persists every event before relaying it, so reloads replay state.</li>
        <li>A stale sweeper reconciles runs whose judge disappeared.</li>
      </ul>
    </div>
    <div class="rc-flow" aria-label="Submission flow">
      <div class="rc-flow-row">
        <div class="rc-node">Browser<small>REST + SSE</small></div>
        <span class="rc-flow-arrow">→</span>
        <div class="rc-node is-brand">Backend<small>persist + relay</small></div>
      </div>
      <div class="rc-flow-row">
        <span class="rc-flow-arrow" style="opacity:0">→</span>
        <div class="rc-node" style="margin-left: 25%;">PostgreSQL<small>durable queue</small></div>
        <div class="rc-node">SeaweedFS<small>source + cases</small></div>
      </div>
      <div class="rc-flow-row">
        <span class="rc-flow-arrow" style="opacity:0">→</span>
        <div class="rc-node is-brand">Judge<small>claim + execute</small></div>
        <span class="rc-flow-arrow">→</span>
        <div class="rc-node">Runner<small>no network</small></div>
      </div>
    </div>
  </div>
</div>

<div class="rc-cta">
  <div class="rc-cta-inner">
    <div>
      <h3>New to the project?</h3>
      <p>Start with the overview, then bring up the stack locally in about ten minutes.</p>
    </div>
    <div class="actions">
      <a class="rc-button" href="./guide/overview.html">What is RunCodes?</a>
      <a class="rc-button secondary" href="./guide/getting-started.html">Quick start</a>
    </div>
  </div>
</div>
