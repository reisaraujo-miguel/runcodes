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

	// Timings and resources. A nil phase means the report has no measurements
	// for it yet (older runs, or a phase the language does not have).
	Timings LanguageTimings `json:"timings"`
	// Memory is the paired peak-memory comparison. It is nil when no result has
	// a known memory figure.
	Memory *MemoryStat `json:"memory,omitempty"`

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

// PhaseStat is the median and spread of one measured phase. The median is null
// when the phase has samples but none of the usable kind (for example an image
// that was only ever cold-pulled); the counts and coverage are still useful.
type PhaseStat struct {
	MedianSeconds        *float64 `json:"median_seconds"`
	P25Seconds           *float64 `json:"p25_seconds"`
	P75Seconds           *float64 `json:"p75_seconds"`
	CPUTimeMedianSeconds *float64 `json:"cpu_time_median_seconds,omitempty"`
	// MemPeakBytes is the compile phase's median peak memory. It is populated
	// only for phases whose monitor report carried a memory figure.
	MemPeakBytes *int64 `json:"mem_peak_bytes,omitempty"`
	Samples      int64  `json:"samples"`
	// WarmSamples and ColdSamples are populated for image_ensure; a cold pull
	// is excluded from the median so image-cache state does not dominate it.
	WarmSamples int64   `json:"warm_samples,omitempty"`
	ColdSamples int64   `json:"cold_samples,omitempty"`
	Coverage    float64 `json:"coverage"`
}

// LanguageTimings is the time breakdown of one language's runs.
type LanguageTimings struct {
	Queue     *PhaseStat `json:"queue,omitempty"`
	Workspace *PhaseStat `json:"workspace,omitempty"`
	Image     *PhaseStat `json:"image,omitempty"`
	Startup   *PhaseStat `json:"startup,omitempty"`
	Compile   *PhaseStat `json:"compile,omitempty"`
	Run       *PhaseStat `json:"run,omitempty"`
	Grade     *PhaseStat `json:"grade,omitempty"`
	Artifact  *PhaseStat `json:"artifact,omitempty"`
}

// MemoryStat is the paired peak-memory comparison. Relative is the median,
// across comparable test cases, of the language's median peak divided by the
// median of the languages on that case. Coverage is the share of executed case
// results with a known memory figure.
type MemoryStat struct {
	Relative          *float64 `json:"relative"`
	P25               *float64 `json:"p25"`
	P75               *float64 `json:"p75"`
	MedianBytes       *int64   `json:"median_bytes"`
	ComparedTestCases int64    `json:"compared_test_cases"`
	ComparedExercises int64    `json:"compared_exercises"`
	KnownResults      int64    `json:"known_results"`
	TotalResults      int64    `json:"total_results"`
	Coverage          float64  `json:"coverage"`
}
