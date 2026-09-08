// A single, consistent shape for "a real request failed" across the app -
// message plus an optional retry action - instead of each page inventing
// its own bare error paragraph. Chat's per-message error bubbles are the
// one deliberate exception (see Brick 20's doc): a failed answer is a
// conversational turn, not a page-level query failure, and re-asking the
// question already serves as its retry.
interface ErrorStateProps {
  message: string;
  onRetry?: () => void;
}

export default function ErrorState({ message, onRetry }: ErrorStateProps) {
  return (
    <div className="flex flex-wrap items-center gap-3 border border-danger/50 bg-danger/10 px-3 py-2.5 text-sm">
      <span className="text-danger">{message}</span>
      {onRetry && (
        <button
          type="button"
          onClick={onRetry}
          className="shrink-0 border border-line-faint px-2 py-1 text-xs text-ink-dim transition-colors hover:border-accent hover:text-ink"
        >
          Retry
        </button>
      )}
    </div>
  );
}
