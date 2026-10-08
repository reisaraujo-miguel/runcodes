import { ArrowRightIcon, CalendarClockIcon } from "lucide-react";
import { NavLink } from "react-router";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import type { OpenExercise } from "@/lib/api/enrollments";
import { deadlineInfo } from "@/lib/deadline";
import { formatDateTime } from "@/lib/format";

/** A single open exercise, ready to submit to. */
export function ExerciseCard({ exercise }: { exercise: OpenExercise }) {
  const deadline = deadlineInfo(exercise.deadline);

  return (
    <Card>
      <CardContent className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="min-w-0 space-y-1.5">
          <div className="flex flex-wrap items-center gap-2">
            <p className="truncate font-medium">{exercise.title}</p>
            <Badge variant={deadline.tone === "neutral" ? "outline" : deadline.tone === "warning" ? "warning" : "destructive"}>
              <CalendarClockIcon />
              {deadline.label}
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
        </div>
        <Button
          className="shrink-0"
          render={<NavLink to={`/exercises/${String(exercise.id)}/submit`} />}
        >
          Resolver
          <ArrowRightIcon data-icon="inline-end" />
        </Button>
      </CardContent>
    </Card>
  );
}
