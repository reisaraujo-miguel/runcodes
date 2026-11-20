import { describe, expect, test } from "bun:test";

import {
  applyArtifactStatus,
  artifactUnavailableMessage,
  pendingUploads,
  type ArtifactStatus,
} from "./artifact-progress";

describe("applyArtifactStatus", () => {
  test("records pending, ready and failed states per case", () => {
    let statuses = new Map();
    statuses = applyArtifactStatus(statuses, {
      test_case_id: 3,
      state: "pending",
    });
    expect(statuses.get(3)).toEqual({
      state: "pending",
      reason: undefined,
      limitBytes: undefined,
    });

    statuses = applyArtifactStatus(statuses, {
      test_case_id: 3,
      state: "failed",
      reason: "commit_artifact_budget_exceeded",
      limit_bytes: 1024,
    });
    expect(statuses.get(3)).toEqual({
      state: "failed",
      reason: "commit_artifact_budget_exceeded",
      limitBytes: 1024,
    });
  });

  test("ignores run-level and malformed events", () => {
    const statuses = new Map<number, ArtifactStatus>();
    expect(applyArtifactStatus(statuses, { state: "failed" })).toEqual(
      statuses,
    );
    expect(
      applyArtifactStatus(statuses, { test_case_id: 3, state: "other" }),
    ).toEqual(statuses);
  });
});

describe("pendingUploads", () => {
  test("returns only cases still uploading", () => {
    const statuses = new Map<number, ArtifactStatus>([
      [1, { state: "pending" }],
      [2, { state: "ready" }],
      [3, { state: "failed" }],
    ]);

    expect(pendingUploads(statuses)).toEqual([1]);
  });
});

describe("artifactUnavailableMessage", () => {
  test("names the commit budget and its limit", () => {
    expect(
      artifactUnavailableMessage({
        state: "failed",
        reason: "commit_artifact_budget_exceeded",
        limitBytes: 1024,
      }),
    ).toContain("limite total");
  });

  test("says when the case produced nothing to download", () => {
    expect(
      artifactUnavailableMessage({
        state: "failed",
        reason: "nothing_to_publish",
      }),
    ).toContain("nenhum arquivo");
  });

  test("explains a failed storage upload", () => {
    expect(
      artifactUnavailableMessage({ state: "failed", reason: "upload_failed" }),
    ).toContain("falhou o envio");
  });

  test("stays silent when the upload is not a failure", () => {
    expect(artifactUnavailableMessage({ state: "pending" })).toBeNull();
    expect(artifactUnavailableMessage({ state: "ready" })).toBeNull();
    expect(artifactUnavailableMessage(undefined)).toBeNull();
  });

  test("still explains an unrecognised failure", () => {
    expect(
      artifactUnavailableMessage({ state: "failed", reason: "new_reason" }),
    ).toContain("não estão disponíveis");
  });
});
