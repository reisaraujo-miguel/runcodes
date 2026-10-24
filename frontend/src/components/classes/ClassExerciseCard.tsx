import {
  CalendarClockIcon,
  EyeIcon,
  SendIcon,
  type LucideIcon,
} from "lucide-react";
import { Link, useLocation } from "react-router";

import { StatusBadge } from "@/components/app/StatusBadge";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import type { UserOfferingExercise } from "@/lib/api";
import { isExerciseOpen, isSubmissionPending } from "@/lib/class-exercises";
import { deadlineInfo } from "@/lib/deadline";
import { formatDateTime } from "@/lib/format";
import { commitStatus } from "@/lib/submission-status";

function submissionCountLabel(count: number): string {
  return count === 1 ? "1 submissão" : `${String(count)} submissões`;
}

/**
 * One exercise on the student-facing class page: when it closes, the caller's
 * best score and latest submission state, and a link to solve (or review) it.
 */
export function ClassExerciseCard({
  exercise,
  now,
}: {
  exercise: UserOfferingExercise;
  /** Current timestamp, owned by the page so this component stays pure. */
  now: number;
}) {
  const location = useLocation();
  const origin = `${location.pathname}${location.search}`;
  const open = isExerciseOpen(exercise, now);
  const pending = isSubmissionPending(exercise);
  const submitted = exercise.submission_count > 0;
  const lastStatus = exercise.last_status;
  const deadline = deadlineInfo(exercise.deadline, now);

  const actionLabel = open
    ? submitted
      ? "Enviar novamente"
      : "Enviar solução"
    : "Ver exercício";
  const ActionIcon: LucideIcon = open ? SendIcon : EyeIcon;

  const scoreHint = pending
    ? "Em correção"
    : submitted
      ? submissionCountLabel(exercise.submission_count)
      : "Sem submissão";

  return (
    <Card>
      <CardContent className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="min-w-0 space-y-2">
          <div className="flex flex-wrap items-center gap-2">
            <p className="truncate font-medium">{exercise.title}</p>
            <Badge
              variant={deadline.tone === "neutral" ? "outline" : deadline.tone}
            >
              <CalendarClockIcon />
              {deadline.label}
            </Badge>
            {lastStatus !== null && lastStatus !== "completed" ? (
              <StatusBadge value={commitStatus(lastStatus)} />
            ) : null}
          </div>

          {exercise.description ? (
            <p className="text-muted-foreground line-clamp-2 text-sm">
              {exercise.description}
            </p>
          ) : null}

          <p className="text-muted-foreground text-xs">
            Abertura: {formatDateTime(exercise.open_date)}
            {" · "}
            Prazo: {formatDateTime(exercise.deadline)}
          </p>
          {exercise.last_submission_at ? (
            <p className="text-muted-foreground text-xs">
              Última entrega em {formatDateTime(exercise.last_submission_at)}
            </p>
          ) : null}
        </div>

        <div className="flex shrink-0 items-center gap-3 sm:flex-col sm:items-end">
          <div className="bg-muted/50 rounded-lg px-3 py-2 text-right">
            <p className="text-muted-foreground text-[10px] font-medium tracking-wide uppercase">
              Melhor nota
            </p>
            <p className="text-xl leading-tight font-semibold tabular-nums">
              {exercise.best_score !== null
                ? exercise.best_score.toFixed(2)
                : "—"}
            </p>
            <p className="text-muted-foreground text-[11px]">{scoreHint}</p>
          </div>

          <Button
            size="sm"
            variant={open ? "default" : "outline"}
            render={
              <Link
                to={`/exercises/${String(exercise.id)}/submit`}
                state={{ from: origin }}
              />
            }
          >
            <ActionIcon />
            {actionLabel}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
