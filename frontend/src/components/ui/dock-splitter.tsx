import { GripHorizontalIcon, GripVerticalIcon } from "lucide-react";
import {
  useEffect,
  useRef,
  useState,
  type KeyboardEvent,
  type PointerEvent,
} from "react";

import { DOCK_SPLITTER_SIZE } from "@/lib/dock-layout";
import { clamp, cn } from "@/lib/utils";

export type DockOrientation = "vertical" | "horizontal";

export interface DockSplitterProps {
  /** `vertical` is a vertical bar that resizes width; `horizontal` resizes height. */
  orientation: DockOrientation;
  /** Share of the space taken by the pane before the splitter. */
  value: number;
  min: number;
  max: number;
  onChange: (value: number) => void;
  /** Restores the default share; wired to double-click. */
  onReset: () => void;
  label: string;
  className?: string;
}

interface DragState {
  pointerId: number;
  startPosition: number;
  startValue: number;
  /** Pixels available to both panes, excluding the splitter itself. */
  available: number;
}

const KEYBOARD_STEP = 0.02;
const KEYBOARD_LARGE_STEP = 0.1;

/**
 * A draggable divider between two docks.
 *
 * Sizes are expressed as the first pane's share of the available space, so the
 * layout keeps its proportions when the window changes. The parent provides
 * the pixel-based min/max ratios; the splitter only enforces them.
 */
export function DockSplitter({
  orientation,
  value,
  min,
  max,
  onChange,
  onReset,
  label,
  className,
}: DockSplitterProps) {
  const [dragging, setDragging] = useState(false);
  const dragRef = useRef<DragState | null>(null);

  useEffect(() => {
    if (!dragging) return;

    const previousCursor = document.body.style.cursor;
    const previousSelect = document.body.style.userSelect;
    document.body.style.cursor =
      orientation === "vertical" ? "col-resize" : "row-resize";
    document.body.style.userSelect = "none";

    return () => {
      document.body.style.cursor = previousCursor;
      document.body.style.userSelect = previousSelect;
    };
  }, [dragging, orientation]);

  function handlePointerDown(event: PointerEvent<HTMLDivElement>) {
    if (event.button !== 0) return;
    const parent = event.currentTarget.parentElement;
    if (!parent) return;

    const available =
      orientation === "vertical"
        ? parent.clientWidth - event.currentTarget.offsetWidth
        : parent.clientHeight - event.currentTarget.offsetHeight;
    if (available <= 0) return;

    event.preventDefault();
    event.currentTarget.focus();
    event.currentTarget.setPointerCapture(event.pointerId);
    dragRef.current = {
      pointerId: event.pointerId,
      startPosition: orientation === "vertical" ? event.clientX : event.clientY,
      startValue: value,
      available,
    };
    setDragging(true);
  }

  function handlePointerMove(event: PointerEvent<HTMLDivElement>) {
    const drag = dragRef.current;
    if (drag?.pointerId !== event.pointerId) return;

    const position = orientation === "vertical" ? event.clientX : event.clientY;
    const next = clamp(
      drag.startValue + (position - drag.startPosition) / drag.available,
      min,
      max,
    );
    onChange(next);
  }

  function endDrag(event: PointerEvent<HTMLDivElement>) {
    if (dragRef.current?.pointerId !== event.pointerId) return;

    dragRef.current = null;
    setDragging(false);
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
  }

  function handleKeyDown(event: KeyboardEvent<HTMLDivElement>) {
    if (event.key === "Home") {
      event.preventDefault();
      onChange(min);
      return;
    }
    if (event.key === "End") {
      event.preventDefault();
      onChange(max);
      return;
    }

    const step = event.shiftKey ? KEYBOARD_LARGE_STEP : KEYBOARD_STEP;
    let delta = 0;
    if (orientation === "vertical") {
      if (event.key === "ArrowLeft") delta = -step;
      else if (event.key === "ArrowRight") delta = step;
    } else if (event.key === "ArrowUp") {
      delta = -step;
    } else if (event.key === "ArrowDown") {
      delta = step;
    }
    if (delta === 0) return;

    event.preventDefault();
    onChange(clamp(value + delta, min, max));
  }

  const percent = Math.round(value * 100);
  const Icon =
    orientation === "vertical" ? GripVerticalIcon : GripHorizontalIcon;

  return (
    <div
      role="separator"
      aria-orientation={orientation}
      aria-label={label}
      aria-valuemin={Math.round(min * 100)}
      aria-valuemax={Math.round(max * 100)}
      aria-valuenow={percent}
      aria-valuetext={`${String(percent)}%`}
      tabIndex={0}
      title="Arraste para redimensionar. Duplo clique para restaurar."
      data-slot="dock-splitter"
      data-dragging={dragging ? "true" : undefined}
      onPointerDown={handlePointerDown}
      onPointerMove={handlePointerMove}
      onPointerUp={endDrag}
      onPointerCancel={endDrag}
      onLostPointerCapture={endDrag}
      onDoubleClick={onReset}
      onKeyDown={handleKeyDown}
      style={
        orientation === "vertical"
          ? { width: DOCK_SPLITTER_SIZE }
          : { height: DOCK_SPLITTER_SIZE }
      }
      className={cn(
        "group relative flex shrink-0 touch-none items-center justify-center outline-none transition-colors",
        orientation === "vertical"
          ? "h-full cursor-col-resize"
          : "w-full cursor-row-resize",
        "hover:bg-primary/5 focus-visible:ring-ring/50 focus-visible:ring-2 data-[dragging=true]:bg-primary/10",
        className,
      )}
    >
      <Icon
        className={cn(
          "text-muted-foreground size-4 transition-colors",
          "group-hover:text-primary group-focus-visible:text-primary group-data-[dragging=true]:text-primary",
        )}
        aria-hidden
      />
    </div>
  );
}
