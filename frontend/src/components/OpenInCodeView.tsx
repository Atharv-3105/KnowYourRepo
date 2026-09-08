import { useCodeView } from "../lib/useCodeView";
import { repoRelativePath } from "../lib/path";

interface OpenInCodeViewProps {
  repoId: string;
  /** Raw backend file_path (e.g. "..\data\repos\<repoID>\hey.go") - cleaned internally. */
  filePath: string;
  startLine?: number;
  endLine?: number;
}

// A small, deliberately separate trigger from CopyableLocation - kept the
// existing, already-tested copy affordance untouched everywhere it's used
// rather than risk regressing it by folding "open" into the same
// component across four different call sites.
export default function OpenInCodeView({ repoId, filePath, startLine, endLine }: OpenInCodeViewProps) {
  const { openFile } = useCodeView();

  return (
    <button
      type="button"
      onClick={() => openFile({ path: repoRelativePath(filePath, repoId), startLine, endLine })}
      className="border border-line-faint px-1.5 py-0.5 text-xs text-ink-dim transition-colors hover:border-accent hover:text-ink"
    >
      open
    </button>
  );
}
