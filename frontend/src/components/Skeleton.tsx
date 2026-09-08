// Static placeholder shapes for loading states - deliberately not a
// shimmer/pulse animation (the frontend-design skill names per-card pulse
// as one of the commonest generic-AI-generated tells). A still shape
// communicates "content is coming, here's its rough size" without adding
// motion that would need its own prefers-reduced-motion handling on top
// of the app's already-restrained animation vocabulary.
interface SkeletonProps {
  className?: string;
}

export function SkeletonLine({ className = "" }: SkeletonProps) {
  return <div className={`h-3.5 bg-line-faint/30 ${className}`} />;
}

export function SkeletonBlock({ className = "" }: SkeletonProps) {
  return <div className={`bg-line-faint/30 ${className}`} />;
}
