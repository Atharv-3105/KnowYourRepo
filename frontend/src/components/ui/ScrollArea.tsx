import * as RadixScrollArea from "@radix-ui/react-scroll-area";
import type { ReactNode } from "react";

interface ScrollAreaProps {
  children: ReactNode;
  className?: string;
}

// A thin scrollbar that only appears on hover/scroll (Radix's own
// behavior) instead of the browser's default chunky scrollbar - used
// anywhere content genuinely needs to scroll within a fixed region
// (Command Palette results, Code View, Symbol Explorer's list).
export function ScrollArea({ children, className = "" }: ScrollAreaProps) {
  return (
    <RadixScrollArea.Root className={`overflow-hidden ${className}`}>
      <RadixScrollArea.Viewport className="h-full w-full">{children}</RadixScrollArea.Viewport>
      <RadixScrollArea.Scrollbar
        orientation="vertical"
        className="flex w-2 touch-none select-none bg-transparent p-0.5"
      >
        <RadixScrollArea.Thumb className="flex-1 bg-line-faint" />
      </RadixScrollArea.Scrollbar>
    </RadixScrollArea.Root>
  );
}
