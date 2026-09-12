export { checkAuth, login, refreshSession, signUp } from "./auth";
export type {
  AuthUser,
  LoginPayload,
  LoginResponse,
  SignUpPayload,
  UserRole,
} from "./auth";

export { createOffering, getOffering } from "./offerings";
export type { CreateOfferingPayload, Offering } from "./offerings";

export {
  createCompilationFile,
  createExercise,
  createTestCase,
  deleteCompilationFile,
  deleteExercise,
  deleteTestCase,
  getAllowedFileTypes,
  getCompilationFiles,
  getExercise,
  getExerciseTestCases,
  getOfferingExercises,
  updateExercise,
} from "./exercises";
export type {
  AllowedFileType,
  CompilationFile,
  Exercise,
  ExercisePayload,
  NewTestCase,
  TestCase,
} from "./exercises";

export { createSubmission, subscribeSubmissionEvents } from "./submissions";
export type {
  CaseResultStatus,
  CommitStatus,
  FinishedStatus,
  QueuedSubmission,
  SubmissionArtifactEvent,
  SubmissionCaseResultEvent,
  SubmissionCommit,
  SubmissionCompilationEvent,
  SubmissionEvent,
  SubmissionEventHandlers,
  SubmissionFinishedEvent,
  SubmissionSnapshotEvent,
  SubmissionSnapshotResult,
  SubmissionStatusEvent,
  SubmissionStreamErrorEvent,
} from "./submissions";
