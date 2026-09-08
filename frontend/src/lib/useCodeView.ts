import { useContext } from "react";
import { CodeViewContext, type CodeViewContextValue } from "./codeViewContext";

export function useCodeView(): CodeViewContextValue {
  const ctx = useContext(CodeViewContext);
  if (!ctx) {
    throw new Error("useCodeView must be used within a CodeViewProvider");
  }
  return ctx;
}
