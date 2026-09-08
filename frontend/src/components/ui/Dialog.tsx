import * as RadixDialog from "@radix-ui/react-dialog";
import { motion } from "motion/react";
import type { ReactNode } from "react";
import { dialogScale, slideInPanel } from "../../lib/motion";

interface DialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  children: ReactNode;
  /** center: a command-palette-style modal. panel: a full-height slide-in from the right (Code View). */
  variant?: "center" | "panel";
  /** Screen-reader-only title - Radix requires one for accessibility even
   * when the dialog's own visible content makes the purpose obvious. */
  srTitle: string;
}

// Shared, blueprint-skinned Radix Dialog. Entrance-only motion (Radix
// unmounts Content immediately on close by default - a real exit
// animation would need forceMount + AnimatePresence, not added here
// since nothing built so far needs it; add it if a later brick does).
export function Dialog({ open, onOpenChange, children, variant = "center", srTitle }: DialogProps) {
  const isPanel = variant === "panel";

  return (
    <RadixDialog.Root open={open} onOpenChange={onOpenChange}>
      <RadixDialog.Portal>
        <RadixDialog.Overlay className="fixed inset-0 z-40 bg-page-deep/70" />
        <RadixDialog.Content
          className={
            isPanel
              ? "fixed inset-y-0 right-0 z-50 w-full max-w-2xl outline-none"
              : "fixed left-1/2 top-24 z-50 w-full max-w-xl -translate-x-1/2 outline-none"
          }
        >
          <RadixDialog.Title className="sr-only">{srTitle}</RadixDialog.Title>
          <motion.div
            initial={isPanel ? slideInPanel.initial : dialogScale.initial}
            animate={isPanel ? slideInPanel.animate : dialogScale.animate}
            transition={isPanel ? slideInPanel.transition : dialogScale.transition}
            className={
              isPanel
                ? "h-full overflow-y-auto border-l border-line-faint bg-page-deep"
                : "border border-line-faint bg-page-deep shadow-2xl"
            }
          >
            {children}
          </motion.div>
        </RadixDialog.Content>
      </RadixDialog.Portal>
    </RadixDialog.Root>
  );
}
