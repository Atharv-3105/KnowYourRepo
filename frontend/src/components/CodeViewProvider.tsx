import { useState, type ReactNode } from "react";
import { CodeViewContext, type CodeViewTarget } from "../lib/codeViewContext";

// Shared across the whole repo-scoped shell (mounted once in RepoLayout)
// so any page - Chat, Architecture, Symbol Explorer, Graph - can open the
// same slide-in Code View panel without navigating away and losing its
// own context underneath. A route-based approach (navigating to a
// "/file" page) would unmount whichever page opened it, defeating the
// "calling context stays visible" point of a slide-in panel.
export function CodeViewProvider({ children }: { children: ReactNode }) {
  const [target, setTarget] = useState<CodeViewTarget | null>(null);

  return (
    <CodeViewContext.Provider value={{ target, openFile: setTarget, close: () => setTarget(null) }}>
      {children}
    </CodeViewContext.Provider>
  );
}
