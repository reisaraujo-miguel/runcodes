import { Loader2Icon } from "lucide-react";

import { cn } from "@/lib/utils";

/** An accessible spinning indicator, sized to the surrounding text. */
function Spinner({
  className,
  label = "Carregando",
  ...props
}: React.ComponentProps<"span"> & { label?: string }) {
  return (
    <span
      role="status"
      aria-label={label}
      data-slot="spinner"
      className={cn("inline-flex", className)}
      {...props}
    >
      <Loader2Icon aria-hidden className="size-4 animate-spin" />
    </span>
  );
}

export { Spinner };
