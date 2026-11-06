import { cva, type VariantProps } from "class-variance-authority";
import { XIcon } from "lucide-react";
import * as React from "react";

import { cn } from "@/lib/utils";

const alertVariants = cva(
  "relative grid w-full grid-cols-[0_1fr] items-start gap-x-3 gap-y-1 rounded-lg border px-4 py-3 text-sm has-[>svg]:grid-cols-[calc(var(--spacing)*4)_1fr] [&>svg]:size-4 [&>svg]:translate-y-0.5 [&>svg]:text-current",
  {
    variants: {
      variant: {
        default: "bg-card text-card-foreground",
        info: "border-info/30 bg-info/10 text-foreground [&>svg]:text-info",
        success:
          "border-success/30 bg-success/10 text-foreground [&>svg]:text-success",
        warning:
          "border-warning/40 bg-warning/10 text-foreground [&>svg]:text-warning",
        destructive:
          "border-destructive/30 bg-destructive/10 text-foreground [&>svg]:text-destructive",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  },
);

function Alert({
  className,
  variant,
  dismissible,
  children,
  ...props
}: React.ComponentProps<"div"> &
  VariantProps<typeof alertVariants> & {
    /** Info alerts can be hidden by default; pass false for a permanent one. */
    dismissible?: boolean;
  }) {
  const [visible, setVisible] = React.useState(true);
  const canDismiss = dismissible ?? variant === "info";

  if (!visible) return null;

  return (
    <div
      role="alert"
      data-slot="alert"
      className={cn(
        alertVariants({ variant }),
        canDismiss && "pr-10",
        className,
      )}
      {...props}
    >
      {children}
      {canDismiss ? (
        <button
          type="button"
          aria-label="Ocultar informação"
          title="Ocultar informação"
          onClick={() => {
            setVisible(false);
          }}
          className="absolute top-2 right-2 rounded p-1 text-muted-foreground transition-colors outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50"
        >
          <XIcon className="size-3.5" aria-hidden />
        </button>
      ) : null}
    </div>
  );
}

function AlertTitle({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="alert-title"
      className={cn(
        "col-start-2 line-clamp-1 min-h-4 font-medium tracking-tight",
        className,
      )}
      {...props}
    />
  );
}

function AlertDescription({
  className,
  ...props
}: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="alert-description"
      className={cn(
        "col-start-2 grid justify-items-start gap-1 text-sm text-muted-foreground [&_p]:leading-relaxed",
        className,
      )}
      {...props}
    />
  );
}

export { Alert, AlertDescription, AlertTitle };
