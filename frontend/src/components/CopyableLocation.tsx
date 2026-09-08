import { useState } from "react";

interface CopyableLocationProps {
  filePath: string;
  startLine?: number;
  endLine?: number;
}

// There's no source-file viewer anywhere in this project (no route or
// backend endpoint serves file content), so a "clickable" file:line can't
// actually link anywhere real. Making it copy-to-clipboard instead is a
// genuinely useful, honest interaction - not a link that goes nowhere.
export default function CopyableLocation({ filePath, startLine, endLine }: CopyableLocationProps) {
  const [copied, setCopied] = useState(false);

  const label = startLine
    ? `${filePath}:${startLine}${endLine && endLine !== startLine ? `-${endLine}` : ""}`
    : filePath;

  const handleClick = async () => {
    try {
      await navigator.clipboard.writeText(label);
      setCopied(true);
      setTimeout(() => setCopied(false), 1200);
    } catch {
      // Clipboard access can be denied by the browser - fail silently,
      // the label is still visible and selectable by hand either way.
    }
  };

  return (
    <button
      type="button"
      onClick={handleClick}
      title="Copy path"
      className="border border-line-faint px-1.5 py-0.5 font-mono text-xs text-ink-dim transition-colors hover:border-accent hover:text-ink"
    >
      {copied ? "copied" : label}
    </button>
  );
}
