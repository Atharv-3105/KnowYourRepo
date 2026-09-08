import type { BundledLanguage } from "shiki";

// Maps a file extension to a Shiki bundled-language id. Deliberately not
// exhaustive - falls back to plain text for anything unrecognized rather
// than guessing.
const EXT_TO_LANG: Record<string, BundledLanguage | "text"> = {
  go: "go",
  py: "python",
  js: "javascript",
  jsx: "jsx",
  ts: "typescript",
  tsx: "tsx",
  json: "json",
  yaml: "yaml",
  yml: "yaml",
  md: "markdown",
  sh: "bash",
  bash: "bash",
  rs: "rust",
  java: "java",
  c: "c",
  h: "c",
  cpp: "cpp",
  hpp: "cpp",
  rb: "ruby",
  php: "php",
  html: "html",
  css: "css",
  sql: "sql",
  toml: "toml",
  mod: "text",
  sum: "text",
};

export function languageFromPath(path: string): BundledLanguage | "text" {
  const ext = path.split(".").pop()?.toLowerCase() ?? "";
  return EXT_TO_LANG[ext] ?? "text";
}
