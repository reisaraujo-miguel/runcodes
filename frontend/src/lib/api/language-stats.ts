import { apiGet } from "./client";

/** One language/format with its adoption, allowance and CPU statistics. */
export interface LanguageStat {
  file_type_id: number;
  name: string;
  extension: string;
  is_compilable: boolean;
  is_available: boolean;
  /** Classified student submissions. */
  submissions: number;
  /** Percentage of all classified student submissions. */
  submission_share: number;
  students: number;
  exercises_chosen: number;
  /** Percentage of submissions in the exercises that allowed this type. */
  pick_rate: number | null;
  /** Number of exercises behind `pick_rate`. */
  pick_rate_samples: number;
  exercises_offered: number;
  exercise_offer_rate: number;
  offerings_offered: number;
  professors_offering: number;
  professor_offer_rate: number;
  results: number;
  test_cases: number;
  compared_test_cases: number;
  compared_exercises: number;
  /** Median CPU seconds on the compared test cases. */
  cpu_time_median_seconds: number | null;
  /** Median ratio against the other languages on the same test cases. */
  relative_cpu: number | null;
  relative_cpu_p25: number | null;
  relative_cpu_p75: number | null;
}

export interface LanguageStatisticsSummary {
  exercises: number;
  professors: number;
  catalog_languages: number;
  languages_offered: number;
  languages_used: number;
  classified_submissions: number;
  students: number;
  unclassified_submissions: number;
}

export interface LanguageStatistics {
  generated_at: string;
  summary: LanguageStatisticsSummary;
  languages: LanguageStat[];
}

/** Platform-wide language adoption, allowance and CPU report. */
export function adminGetLanguageStatistics(): Promise<LanguageStatistics> {
  return apiGet<LanguageStatistics>("/api/v1/admin/language-stats");
}

/** The same report scoped to one class, for its owner and co-professors. */
export function getOfferingLanguageStatistics(
  offeringId: number,
): Promise<LanguageStatistics> {
  return apiGet<LanguageStatistics>(
    `/api/v1/offerings/${String(offeringId)}/language-stats`,
  );
}
