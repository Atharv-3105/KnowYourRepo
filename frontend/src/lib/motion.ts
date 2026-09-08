// Shared motion vocabulary - every animated moment introduced in Bricks 15+
// pulls its timing/easing from here instead of inventing its own. The goal
// (per the design brief) is restraint: a small, consistent set of
// durations/curves used deliberately, not a different spring/bounce per
// component. No overshoot, no bounce - every curve here settles, it
// doesn't oscillate.

export const duration = {
  fast: 0.12, // dialog/palette open-close, tooltip reveal
  base: 0.18, // message settle, panel reveal, tab indicator slide
  slow: 0.24, // larger layout shifts - a slide-in panel like Code View
} as const;

// A restrained "ease-out" curve (decelerate into place, no overshoot) -
// used for anything entering or growing. Deliberately not a spring: a
// spring implies physical bounce, which reads as playful/AI-slop for a
// technical tool; this settles once and stops.
export const easeOut = [0.16, 1, 0.3, 1] as const;

// Symmetric ease for things that fade in AND out through the same
// transition (opacity dimming, focus-mode highlighting).
export const easeInOut = [0.65, 0, 0.35, 1] as const;

// Ready-made variants for the most common recurring moments, so later
// bricks import instead of re-deriving. Each name says what the motion
// communicates, per the brief's "answer what just changed" principle.
export const fadeIn = {
  initial: { opacity: 0 },
  animate: { opacity: 1 },
  exit: { opacity: 0 },
  transition: { duration: duration.base, ease: easeOut },
};

export const settleUp = {
  initial: { opacity: 0, y: 6 },
  animate: { opacity: 1, y: 0 },
  transition: { duration: duration.base, ease: easeOut },
};

export const dialogScale = {
  initial: { opacity: 0, scale: 0.98 },
  animate: { opacity: 1, scale: 1 },
  exit: { opacity: 0, scale: 0.98 },
  transition: { duration: duration.fast, ease: easeOut },
};

export const slideInPanel = {
  initial: { x: "100%" },
  animate: { x: 0 },
  exit: { x: "100%" },
  transition: { duration: duration.slow, ease: easeOut },
};
