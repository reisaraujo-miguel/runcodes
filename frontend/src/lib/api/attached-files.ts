import { apiDelete, apiGet, apiRequest } from "./client";

/** A material attached to an exercise (statement, dataset, starter file). */
export interface AttachedFile {
  id: number;
  exercise_id: number;
  path: string;
  filename: string;
}

/** List the materials attached to an exercise (author or enrolled member). */
export function listAttachedFiles(exerciseId: number): Promise<AttachedFile[]> {
  return apiGet<AttachedFile[]>(
    `/api/v1/exercises/${String(exerciseId)}/attached-files`,
  );
}

/** Attach a material to an exercise. A same-named file is replaced. */
export function createAttachedFile(
  exerciseId: number,
  file: File,
): Promise<AttachedFile> {
  const form = new FormData();
  form.append("file", file);
  return apiRequest<AttachedFile>(
    `/api/v1/exercises/${String(exerciseId)}/attached-files`,
    { method: "POST", body: form },
  );
}

/** Remove a material from an exercise. */
export function deleteAttachedFile(
  exerciseId: number,
  fileId: number,
): Promise<void> {
  return apiDelete(
    `/api/v1/exercises/${String(exerciseId)}/attached-files/${String(fileId)}`,
  );
}
