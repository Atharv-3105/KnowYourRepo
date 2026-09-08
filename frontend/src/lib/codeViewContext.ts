import { createContext } from "react";

export interface CodeViewTarget {
  /** Repo-relative path (already cleaned via repoRelativePath, not a raw backend file_path). */
  path: string;
  startLine?: number;
  endLine?: number;
}

export interface CodeViewContextValue {
  target: CodeViewTarget | null;
  openFile: (target: CodeViewTarget) => void;
  close: () => void;
}

// Split from the Provider component and the useCodeView hook (separate
// files) so each file exports exactly one thing - keeps React Fast
// Refresh able to hot-reload the component/hook independently of this
// plain context object.
export const CodeViewContext = createContext<CodeViewContextValue | null>(null);
