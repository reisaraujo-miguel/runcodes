import {
  ArrowRightIcon,
  CalendarClockIcon,
  CheckCircle2Icon,
  CircleDashedIcon,
  CirclePercentIcon,
  ClockIcon,
  EyeIcon,
  type LucideIcon,
} from "lucide-react";
import { NavLink, useLocation } from "react-router";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import type { UserExercise } from "@/lib/api/enrollments";
import { deadlineInfo } from "@/lib/deadline";
import {
  exerciseProgress,
  type ExerciseProgress,
  type ExerciseProgressState,
} from "@/lib/exercise-progress";
import { formatDateTime } from "@/lib/format";

const PROGRESS_VARIANT: Record<
  ExerciseProgress["tone"],
  "outline" | "info" | "success" | "warning"
> = {
  neutral: "outline",
  info: "info",
  success: "success",
  warning: "warning",
};

const PROGRESS_ICON: Record<ExerciseProgressState, LucideIcon> = {
  passed: CheckCircle2Icon,
  scored: CirclePercentIcon,
  pending: ClockIcon,
  submitted: CircleDashedIcon,
  unsubmitted: CircleDashedIcon,
};

/**
 * One exercise in the home and exercises lists. Besides the deadline, the card
 * states the caller's own progress explicitly — "Sem submissão", a partial
 * "<score>/100", "Em correção" or a full "100/100" — so a submitted exercise
 * never looks identical to one that was never opened.
 */
export function ExerciseCard({ exercise }: { exercise: UserExercise }) {
  const location = useLocation();
  const origin = `${location.pathname}${location.search}`;
  const deadline = deadlineInfo(exercise.deadline);
  const progress = exerciseProgress(exercise);
  const ProgressIcon = PROGRESS_ICON[progress.state];
  const reviewOnly = progress.passed || !exercise.is_open;

  return (
    <Card>
      <CardContent className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="min-w-0 space-y-1.5">
          <div className="flex flex-wrap items-center gap-2">
            <p className="truncate font-medium">{exercise.title}</p>
            <Badge
              variant={
                progress.passed
                  ? "outline"
                  : deadline.tone === "neutral"
                    ? "outline"
                    : deadline.tone === "warning"
                      ? "warning"
                      : "destructive"
              }
            >
              <CalendarClockIcon />
              {deadline.label}
            </Badge>
            <Badge variant={PROGRESS_VARIANT[progress.tone]}>
              <ProgressIcon />
              {progress.label}
            </Badge>
          </div>
          <p className="text-muted-foreground text-sm">
            {exercise.offering_name}
          </p>
          {exercise.description ? (
            <p className="text-muted-foreground line-clamp-2 text-sm">
              {exercise.description}
            </p>
          ) : null}
          <p className="text-muted-foreground text-xs">
            Prazo: {formatDateTime(exercise.deadline)}
          </p>
          {exercise.last_submission_at ? (
            <p className="text-muted-foreground text-xs">
              Última entrega em {formatDateTime(exercise.last_submission_at)}
            </p>
          ) : null}
        </div>
        <Button
          className="shrink-0"
          variant={reviewOnly ? "outline" : "default"}
          render={
            <NavLink
              to={`/exercises/${String(exercise.id)}/submit`}
              state={{ from: origin }}
            />
          }
        >
          {reviewOnly ? <EyeIcon /> : null}
          {reviewOnly ? "Ver exercício" : "Resolver"}
          {reviewOnly ? null : <ArrowRightIcon data-icon="inline-end" />}
        </Button>
      </CardContent>
    </Card>
  );
}
