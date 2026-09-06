import { useMutation } from "@tanstack/react-query";
import { useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { agentChat, ApiError } from "../api";
import type { Source } from "../api";
import CopyableLocation from "../components/CopyableLocation";

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

  return (
    <div className="flex flex-col">
      <div className="space-y-3">
        {messages.length === 0 && (
          <p className="text-sm text-line-faint">Ask a question about this repository.</p>
        )}

        {messages.map((m, i) => (
          <div key={i} className={m.role === "user" ? "flex justify-end" : "flex justify-start"}>
            <div
              className={`max-w-[85%] border-y px-4 py-3 text-sm ${
                m.role === "user"
                  ? "border-y-transparent border-r-2 border-r-accent bg-paper-deep"
                  : "border-y-transparent border-l-2 border-l-line-faint bg-paper-deep"
              }`}
            >
              {m.role === "assistant" && m.refreshing && (
                <div className="mb-1.5 text-xs text-accent">
                  Refreshing in the background - this answer may be slightly stale.
                </div>
              )}

              <div className="whitespace-pre-wrap text-line">{m.content}</div>

              {m.role === "assistant" && m.toolsUsed && m.toolsUsed.length > 0 && (
                <div className="mt-2 flex flex-wrap gap-1">
                  {m.toolsUsed.map((tool) => (
                    <span key={tool} className="border border-line-faint px-1.5 py-0.5 font-mono text-xs text-line-dim">
                      {tool}
                    </span>
                  ))}
                </div>
              )}

              {m.role === "assistant" && m.sources && m.sources.length > 0 && (
                <div className="mt-3 border-t border-line-faint pt-2">
                  <div className="mb-1.5 text-xs text-line-dim">Sources</div>
                  <div className="flex flex-wrap gap-1.5">
                    {m.sources.map((s, si) => (
                      <span key={si} className="inline-flex items-center gap-1">
                        <CopyableLocation
                          filePath={s.file_path}
                          startLine={s.start_line}
                          endLine={s.end_line}
                        />
                        <button
                          type="button"
                          onClick={() =>
                            navigate(`/repos/${repoId}/graph?symbol=${encodeURIComponent(s.symbol)}`)
                          }
                          className="border border-line-faint px-1.5 py-0.5 text-xs text-line-dim transition-colors hover:border-accent hover:text-line"
                        >
                          view in graph
                        </button>
                      </span>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </div>
        ))}

        {chatMutation.isPending && <p className="text-sm text-line-dim">Thinking&hellip;</p>}
      </div>

      <form onSubmit={handleSubmit} className="mt-6 flex border border-line-faint">
        <input
          type="text"
          required
          placeholder="Ask a question..."
          value={question}
          onChange={(e) => setQuestion(e.target.value)}
          disabled={chatMutation.isPending}
          className="flex-1 bg-paper-deep px-4 py-2.5 text-sm text-line placeholder:text-line-dim outline-none disabled:opacity-50"
        />
        <button
          type="submit"
          disabled={chatMutation.isPending}
          className="border-l border-line-faint bg-accent px-5 py-2.5 text-sm font-medium text-paper-deep transition-colors hover:bg-accent-dim disabled:opacity-50"
        >
          Send
        </button>
      </form>

      <p className="mt-2 text-xs text-line-faint">
        Kept in memory on the server only - it won't survive a backend restart, and this
        transcript won't survive a page reload either.
      </p>
    </div>
  );
}
