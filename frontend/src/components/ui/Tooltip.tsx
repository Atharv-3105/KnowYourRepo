import * as RadixTooltip from "@radix-ui/react-tooltip";
import { motion } from "motion/react";
import type { ReactNode } from "react";
import { fadeIn } from "../../lib/motion";

// One shared Provider at the app root (mounted in main.tsx) means every
// Tooltip below shares Radix's hover-delay-group behavior instead of each
// tooltip re-triggering its own full delay.
export function TooltipProvider({ children }: { children: ReactNode }) {
  return <RadixTooltip.Provider delayDuration={300}>{children}</RadixTooltip.Provider>;
}

interface TooltipProps {
  content: ReactNode;
  children: ReactNode;
}

export function Tooltip({ content, children }: TooltipProps) {
  return (
    <RadixTooltip.Root>
      <RadixTooltip.Trigger asChild>{children}</RadixTooltip.Trigger>
      <RadixTooltip.Portal>
        <RadixTooltip.Content
          sideOffset={6}
          className="z-50 border border-line-faint bg-page-deep px-2 py-1 text-xs text-ink"
        >
          <motion.div initial={fadeIn.initial} animate={fadeIn.animate} transition={fadeIn.transition}>
            {content}
          </motion.div>
        </RadixTooltip.Content>
      </RadixTooltip.Portal>
    </RadixTooltip.Root>
  );
}
