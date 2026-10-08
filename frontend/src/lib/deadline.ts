/** A short, urgency-aware description of an exercise deadline. */
export interface DeadlineInfo {
  /** Human label, e.g. "Encerra em 2 dias" or the formatted date. */
  label: string;
  tone: "neutral" | "warning" | "destructive";
  /** True once the deadline is in the past. */
  expired: boolean;
}

const DAY_MS = 24 * 60 * 60 * 1000;

/**
 * Classifies a deadline into a label and a tone: today and overdue are the most
 * urgent, the next three days are a warning, and anything further out is neutral.
 */
export function deadlineInfo(deadline: string, now = Date.now()): DeadlineInfo {
  const time = new Date(deadline).getTime();
  if (Number.isNaN(time)) {
    return { label: deadline, tone: "neutral", expired: false };
  }

  const remaining = time - now;
  if (remaining <= 0) {
    return { label: "Encerrado", tone: "destructive", expired: true };
  }

  if (remaining < DAY_MS) {
    const hours = Math.max(1, Math.round(remaining / (60 * 60 * 1000)));
    return {
      label: `Encerra em ${String(hours)}h`,
      tone: "destructive",
      expired: false,
    };
  }

  const days = Math.ceil(remaining / DAY_MS);
  if (days <= 3) {
    return {
      label: `Encerra em ${String(days)} dia${days > 1 ? "s" : ""}`,
      tone: "warning",
      expired: false,
    };
  }

  return {
    label: new Date(time).toLocaleDateString(undefined, { dateStyle: "medium" }),
    tone: "neutral",
    expired: false,
  };
}
