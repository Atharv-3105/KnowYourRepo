import ReactMarkdown, { type Components } from "react-markdown";

// Renders an LLM answer. react-markdown never renders raw HTML (no
// dangerouslySetInnerHTML), so model output can't inject markup; links are
// forced to open in a new tab without leaking the opener.
const components: Components = {
  p: ({ children }) => <p className="mb-2 last:mb-0">{children}</p>,
  strong: ({ children }) => <strong className="font-semibold">{children}</strong>,
  em: ({ children }) => <em className="italic">{children}</em>,
  h1: ({ children }) => <h3 className="mb-1.5 mt-3 font-display text-base font-medium">{children}</h3>,
  h2: ({ children }) => <h3 className="mb-1.5 mt-3 font-display text-base font-medium">{children}</h3>,
  h3: ({ children }) => <h4 className="mb-1 mt-2 font-medium">{children}</h4>,
  ul: ({ children }) => <ul className="mb-2 list-disc space-y-1 pl-5 last:mb-0">{children}</ul>,
  ol: ({ children }) => <ol className="mb-2 list-decimal space-y-1 pl-5 last:mb-0">{children}</ol>,
  blockquote: ({ children }) => (
    <blockquote className="mb-2 border-l-2 border-line pl-3 text-ink-dim">{children}</blockquote>
  ),
  a: ({ href, children }) => (
    <a href={href} target="_blank" rel="noopener noreferrer" className="text-accent underline underline-offset-2">
      {children}
    </a>
  ),
  pre: ({ children }) => (
    <pre className="mb-2 overflow-x-auto border border-line bg-page p-3 font-mono text-xs leading-relaxed last:mb-0 [&_code]:border-0 [&_code]:bg-transparent [&_code]:p-0 [&_code]:text-[1em]">
      {children}
    </pre>
  ),
  // Inline code gets a chip; the same element inside a fenced block is reset
  // by the [&_code] rules on <pre> above, so fenced blocks need no special-casing.
  code: ({ className, children }) => (
    <code className={`${className ?? ""} border border-line bg-page px-1 py-px font-mono text-[0.85em]`}>{children}</code>
  ),
};

export default function Markdown({ children }: { children: string }) {
  return <ReactMarkdown components={components}>{children}</ReactMarkdown>;
}
