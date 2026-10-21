export {
  checkAuth,
  login,
  logout,
  refreshSession,
  resendVerification,
  signUp,
  verifyEmail,
} from "./auth";
export type {
  AuthUser,
  LoginPayload,
  LoginResponse,
  SignUpPayload,
  UserRole,
} from "./auth";

export { changePassword, getProfile, updateProfile } from "./profile";
export type { PasswordPayload, Profile, ProfilePayload } from "./profile";

export {
  enroll,
  getMyOfferings,
  getMyOpenExercises,
  unenroll,
} from "./enrollments";
export type { OpenExercise, UserOffering } from "./enrollments";

export {
  addOfferingMember,
  createOffering,
  deleteOffering,
  getOffering,
  listOfferingMembers,
  listOfferings,
  removeOfferingMember,
  updateOffering,
  updateOfferingMember,
} from "./offerings";
export type {
  CreateOfferingPayload,
  ManagedOffering,
  MemberRole,
  Offering,
  OfferingMember,
  UpdateOfferingPayload,
} from "./offerings";

export {
  ADMIN_PAGE_LIMIT,
  USER_ROLES,
  adminDeleteOffering,
  adminDeleteUser,
  adminGetSettings,
  adminListOfferingMembers,
  adminListOfferings,
  adminListUsers,
  adminUpdateOffering,
  adminUpdateSettings,
  adminUpdateUser,
} from "./admin";
export type {
  AdminOffering,
  AdminUser,
  UpdateAdminOfferingPayload,
  UpdateUserPayload,
} from "./admin";

export { FALLBACK_SETTINGS, getPublicSettings } from "./settings";
export type { PlatformSettings } from "./settings";

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
  updateTestCase,
} from "./exercises";
export type {
  AllowedFileType,
  CompilationFile,
  Exercise,
  ExercisePayload,
  NewTestCase,
  TestCase,
  TestCasePatch,
} from "./exercises";

export {
  attachedFileDownloadUrl,
  createAttachedFile,
  deleteAttachedFile,
  listAttachedFiles,
} from "./attached-files";
export type { AttachedFile } from "./attached-files";

export {
  createSubmission,
  getSubmissionSourceBlob,
  getSubmissionSourceText,
  listExerciseSubmissions,
  submissionOutputUrl,
  subscribeSubmissionEvents,
} from "./submissions";
export type {
  CaseResultStatus,
  CommitStatus,
  FinishedStatus,
  QueuedSubmission,
  SubmissionArtifact,
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
  SubmissionSummary,
} from "./submissions";
