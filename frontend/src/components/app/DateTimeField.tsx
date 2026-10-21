import type { ReactNode } from "react";

import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";

/**
 * A date input paired with a time input. End-of-period fields can offer the
 * "Fim do dia" shortcut, which fixes the time at 23:59 and disables the time
 * input; unchecking it brings the professor's previous time back.
 */
export function DateTimeField({
  id,
  label,
  date,
  time,
  onDateChange,
  onTimeChange,
  endOfDay,
  onEndOfDayChange,
  disabled = false,
  required = false,
  description,
  className,
}: {
  id: string;
  label: string;
  date: string;
  time: string;
  onDateChange: (value: string) => void;
  onTimeChange: (value: string) => void;
  /** Current end-of-day state; only meaningful when onEndOfDayChange is set. */
  endOfDay?: boolean;
  /** Provides the "Fim do dia" checkbox when set. */
  onEndOfDayChange?: (checked: boolean) => void;
  disabled?: boolean;
  required?: boolean;
  description?: ReactNode;
  className?: string;
}) {
  const isEndOfDay = endOfDay ?? false;
  const timeId = `${id}-time`;

  return (
    <div className={cn("space-y-2", className)}>
      <Label htmlFor={id}>{label}</Label>
      <div className="flex flex-wrap items-center gap-2">
        <Input
          id={id}
          type="date"
          value={date}
          disabled={disabled}
          required={required}
          className="min-w-40 flex-1"
          onChange={(event) => {
            onDateChange(event.target.value);
          }}
        />
        <Input
          id={timeId}
          type="time"
          aria-label={`${label} — horário`}
          value={isEndOfDay ? "23:59" : time}
          disabled={disabled || isEndOfDay}
          required={required && !isEndOfDay}
          className="w-32 font-mono [&::-webkit-calendar-picker-indicator]:ml-2 [&::-webkit-calendar-picker-indicator]:cursor-pointer [&::-webkit-calendar-picker-indicator]:opacity-70"
          onChange={(event) => {
            onTimeChange(event.target.value);
          }}
        />
      </div>

      {onEndOfDayChange ? (
        <Label
          className="text-muted-foreground gap-2 text-xs font-normal"
          htmlFor={`${id}-end-of-day`}
        >
          <Checkbox
            id={`${id}-end-of-day`}
            checked={isEndOfDay}
            disabled={disabled}
            onCheckedChange={(checked) => {
              onEndOfDayChange(checked);
            }}
          />
          Fim do dia (23:59)
        </Label>
      ) : null}

      {description ? (
        <p className="text-muted-foreground text-xs">{description}</p>
      ) : null}
    </div>
  );
}
