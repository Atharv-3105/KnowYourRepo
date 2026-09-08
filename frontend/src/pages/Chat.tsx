import { useMutation } from "@tanstack/react-query";
import { motion } from "motion/react";
import { useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { agentChat, ApiError } from "../api";
import type { Source } from "../api";
import CopyableLocation from "../components/CopyableLocation";
import OpenInCodeView from "../components/OpenInCodeView";
import { duration, easeOut, settleUp } from "../lib/motion";
import { toolBadgeClasses } from "../lib/toolColor";

interface Message {
  role: "user" | "assistant";
  content: string;
  toolsUsed?: string[];
  sources?: Source[];
  refreshing?: boolean;
}

// Per-repo session id, generated once and persisted in localStorage so
// multi-turn conversations keep working against the backend's in-memory
// chat.Store across page reloads (the visible transcript below does NOT
// survive a reload though - there's no GET history endpoint, only the
// backend's own server-side session memory persists).
function getSessionId(repoId: string): string {
  const key = `kyr:session:${repoId}`;
  const existing = localStorage.getItem(key);
  if (existing) return existing;
  const id = crypto.randomUUID();
  localStorage.setItem(key, id);
  return id;
}

export default function Chat() {
  const { repoId } = useParams<{ repoId: string }>();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const [sessionId] = useState(() => getSessionId(repoId!));
  const [messages, setMessages] = useState<Message[]>([]);
  // Initialized once from ?prefill= (arrived here via a "Ask about this"
  // link from the Graph view, Brick 10) - read only at mount, so it
  // doesn't fight the user's own typing on every render.
  const [question, setQuestion] = useState(() => searchParams.get("prefill") ?? "");

  const chatMutation = useMutation({
    // Takes the question as an explicit argument rather than closing over
    // the `question` state variable - mutationFn runs asynchronously, and
    // by the time it does, handleSubmit's setQuestion("") has already
    // cleared that state, so a closure-captured `question` reads back
    // empty. Passing it through mutate(trimmed) below fixes that.
    mutationFn: (question: string) => agentChat({ repo_id: repoId!, question, session_id: sessionId }),
    onSuccess: (res) => {
      setMessages((prev) => [
        ...prev,
        {
          role: "assistant",
          content: res.answer,
          toolsUsed: res.tools_used,
          sources: res.sources,
          refreshing: res.refreshing,
        },
      ]);
    },
    onError: (err) => {
      setMessages((prev) => [
        ...prev,
        {
          role: "assistant",
          content: err instanceof ApiError ? `Error: ${err.message}` : "Something went wrong.",
        },
      ]);
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = question.trim();
    if (!trimmed || chatMutation.isPending) return;

    setMessages((prev) => [...prev, { role: "user", content: trimmed }]);
    setQuestion("");
    chatMutation.mutate(trimmed);
  };

  const jumpTo = (i: number) => {
    document.getElementById(`msg-${i}`)?.scrollIntoView({ behavior: "smooth", block: "start" });
  };

  const questionIndices = messages
    .map((m, i) => ({ ...m, i }))
    .filter((m) => m.role === "user");

  return (
    <div className="flex gap-8">
      <aside className="sticky top-8 hidden w-56 shrink-0 self-start md:block">
        <div className="mb-2 text-xs text-ink-dim">Questions asked</div>
        {questionIndices.length === 0 ? (
          <p className="text-xs text-ink-faint">None yet.</p>
        ) : (
          <ul className="space-y-2 border-t border-line-faint pt-2">
            {questionIndices.map((m) => (
              <li key={m.i}>
                <button
                  type="button"
                  onClick={() => jumpTo(m.i)}
                  className="line-clamp-2 text-left text-xs text-ink-dim hover:text-ink"
                >
                  {m.content}
                </button>
              </li>
            ))}
          </ul>
        )}
      </aside>

      <div className="min-w-0 flex-1">
        <div className="space-y-3">
          {messages.length === 0 && (
            <p className="text-sm text-ink-faint">Ask a question about this repository.</p>
          )}

          {messages.map((m, i) => (
            <motion.div
              key={i}
              id={`msg-${i}`}
              initial={settleUp.initial}
              animate={settleUp.animate}
              transition={settleUp.transition}
              className={m.role === "user" ? "flex justify-end" : "flex justify-start"}
            >
              <div
                className={`max-w-[85%] border-y px-4 py-3 text-sm ${
                  m.role === "user"
                    ? "border-y-transparent border-r-2 border-r-accent bg-page-deep"
                    : "border-y-transparent border-l-2 border-l-line-faint bg-page-deep"
                }`}
              >
                {m.role === "assistant" && m.refreshing && (
                  <div className="mb-1.5 text-xs text-accent">
                    Refreshing in the background - this answer may be slightly stale.
                  </div>
                )}

                <div className="whitespace-pre-wrap text-ink">{m.content}</div>

                {m.role === "assistant" && m.toolsUsed && m.toolsUsed.length > 0 && (
                  <div className="mt-2 flex flex-wrap gap-1">
                    {m.toolsUsed.map((tool) => (
                      <span
                        key={tool}
                        className={`border px-1.5 py-0.5 font-mono text-xs ${toolBadgeClasses(tool)}`}
                      >
                        {tool}
                      </span>
                    ))}
                  </div>
                )}

                {m.role === "assistant" && m.sources && m.sources.length > 0 && (
                  <motion.div
                    initial={{ height: 0, opacity: 0 }}
                    animate={{ height: "auto", opacity: 1 }}
                    transition={{ duration: duration.base, ease: easeOut }}
                    className="mt-3 overflow-hidden border-t border-line-faint pt-2"
                  >
                    <div className="mb-1.5 text-xs text-ink-dim">Sources</div>
                    <ol className="space-y-1.5">
                      {m.sources.map((s, si) => (
                        <li key={si} className="flex flex-wrap items-center gap-1.5">
                          <span className="font-mono text-xs text-ink-faint">
                            {String(si + 1).padStart(2, "0")}
                          </span>
                          <span className="font-mono text-xs text-ink">{s.symbol}</span>
                          <CopyableLocation filePath={s.file_path} startLine={s.start_line} endLine={s.end_line} />
                          <OpenInCodeView
                            repoId={repoId!}
                            filePath={s.file_path}
                            startLine={s.start_line}
                            endLine={s.end_line}
                          />
                          <button
                            type="button"
                            onClick={() =>
                              navigate(`/repos/${repoId}/graph?symbol=${encodeURIComponent(s.symbol)}`)
                            }
                            className="border border-line-faint px-1.5 py-0.5 text-xs text-ink-dim transition-colors hover:border-accent hover:text-ink"
                          >
                            view in graph
                          </button>
                        </li>
                      ))}
                    </ol>
                  </motion.div>
                )}
              </div>
            </motion.div>
          ))}

          {chatMutation.isPending && <p className="text-sm text-ink-dim">Thinking&hellip;</p>}
        </div>

        <form onSubmit={handleSubmit} className="mt-6 flex border border-line-faint">
          <input
            type="text"
            required
            placeholder="Ask a question..."
            value={question}
            onChange={(e) => setQuestion(e.target.value)}
            disabled={chatMutation.isPending}
            className="flex-1 bg-page-deep px-4 py-2.5 text-sm text-ink placeholder:text-ink-dim outline-none disabled:opacity-50"
          />
          <button
            type="submit"
            disabled={chatMutation.isPending}
            className="border-l border-line-faint bg-accent px-5 py-2.5 text-sm font-medium text-page-deep transition-colors hover:bg-accent-dim disabled:opacity-50"
          >
            Send
          </button>
        </form>

        <p className="mt-2 text-xs text-ink-faint">
          Kept in memory on the server only - it won't survive a backend restart, and this
          transcript won't survive a page reload either.
        </p>
      </div>
    </div>
  );
}
