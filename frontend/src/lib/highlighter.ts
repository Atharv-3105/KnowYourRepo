import { createHighlighterCore, type HighlighterCore } from "@shikijs/core";
import { createJavaScriptRegexEngine } from "@shikijs/engine-javascript";
import githubLight from "@shikijs/themes/github-light";
import bash from "@shikijs/langs/bash";
import c from "@shikijs/langs/c";
import cpp from "@shikijs/langs/cpp";
import css from "@shikijs/langs/css";
import go from "@shikijs/langs/go";
import html from "@shikijs/langs/html";
import java from "@shikijs/langs/java";
import javascript from "@shikijs/langs/javascript";
import json from "@shikijs/langs/json";
import jsx from "@shikijs/langs/jsx";
import markdown from "@shikijs/langs/markdown";
import php from "@shikijs/langs/php";
import python from "@shikijs/langs/python";
import ruby from "@shikijs/langs/ruby";
import rust from "@shikijs/langs/rust";
import sql from "@shikijs/langs/sql";
import toml from "@shikijs/langs/toml";
import tsx from "@shikijs/langs/tsx";
import typescript from "@shikijs/langs/typescript";
import yaml from "@shikijs/langs/yaml";

// A hand-curated highlighter core, not the "shiki" package's convenience
// codeToTokens shorthand: that shorthand eagerly bundles its engine plus
// a large default language set into the main chunk (confirmed directly -
// the main bundle grew ~150kB raw after a naive `import { codeToTokens }
// from "shiki"`). createHighlighterCore + explicit per-language imports +
// the JS-regex engine (no WASM oniguruma) only ships exactly the ~20
// languages languageFromPath actually maps to.
let highlighterPromise: Promise<HighlighterCore> | null = null;

export function getHighlighter(): Promise<HighlighterCore> {
  if (!highlighterPromise) {
    highlighterPromise = createHighlighterCore({
      themes: [githubLight],
      langs: [
        bash, c, cpp, css, go, html, java, javascript, json, jsx,
        markdown, php, python, ruby, rust, sql, toml, tsx, typescript, yaml,
      ],
      engine: createJavaScriptRegexEngine(),
    });
  }
  return highlighterPromise;
}
