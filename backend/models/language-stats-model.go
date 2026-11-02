package models

import "time"

// LanguageStatistics is the platform-wide language report shown in the admin
// panel. It combines three views of the same file-type catalog: what students
// choose, what professors allow, and how much CPU the language uses relative to
// the other languages that ran the same test cases.
type LanguageStatistics struct {
	GeneratedAt time.Time                 `json:"generated_at"`
	Summary     LanguageStatisticsSummary `json:"summary"`
	Languages   []LanguageStat            `json:"languages"`
}

// LanguageStatisticsSummary carries the denominators the per-language rates are
// computed against, plus the submissions that could not be attributed to a
// catalog entry (legacy submissions or exercises without allowed types).
type LanguageStatisticsSummary struct {
	Exercises               int64 `json:"exercises"`
	Professors              int64 `json:"professors"`
	CatalogLanguages        int64 `json:"catalog_languages"`
	LanguagesOffered        int64 `json:"languages_offered"`
	LanguagesUsed           int64 `json:"languages_used"`
	ClassifiedSubmissions   int64 `json:"classified_submissions"`
	Students                int64 `json:"students"`
	UnclassifiedSubmissions int64 `json:"unclassified_submissions"`
}

// LanguageStat is one catalog entry with its adoption, allowance and CPU
// statistics. A pointer field is null when the sample does not exist (no
// offered exercises with submissions, or no comparable test cases).
type LanguageStat struct {
	FileTypeID   int64  `json:"file_type_id"`
	Name         string `json:"name"`
	Extension    string `json:"extension"`
	IsCompilable bool   `json:"is_compilable"`
	IsAvailable  bool   `json:"is_available"`

	// Student choice. SubmissionShare and PickRate are percentages.
	Submissions     int64    `json:"submissions"`
	SubmissionShare float64  `json:"submission_share"`
	Students        int64    `json:"students"`
	ExercisesChosen int64    `json:"exercises_chosen"`
	PickRate        *float64 `json:"pick_rate"`
	PickRateSamples int64    `json:"pick_rate_samples"`

	// Professor allowance. Rates are percentages.
	ExercisesOffered   int64   `json:"exercises_offered"`
	ExerciseOfferRate  float64 `json:"exercise_offer_rate"`
	OfferingsOffered   int64   `json:"offerings_offered"`
	ProfessorsOffering int64   `json:"professors_offering"`
	ProfessorOfferRate float64 `json:"professor_offer_rate"`

	// CPU performance. RelativeCPU is the median, across comparable test cases,
	// of this language's median CPU time divided by the median of the languages
	// that ran the same test case. 1.0 is the typical language on that case.
	Results              int64    `json:"results"`
	TestCases            int64    `json:"test_cases"`
	ComparedTestCases    int64    `json:"compared_test_cases"`
	ComparedExercises    int64    `json:"compared_exercises"`
	CPUTimeMedianSeconds *float64 `json:"cpu_time_median_seconds"`
	RelativeCPU          *float64 `json:"relative_cpu"`
	RelativeCPUP25       *float64 `json:"relative_cpu_p25"`
	RelativeCPUP75       *float64 `json:"relative_cpu_p75"`
}
