import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router-dom";
import { ApiError, getArchitecture } from "../api";
import CopyableLocation from "../components/CopyableLocation";

export default function Architecture() {
  const { repoId } = useParams<{ repoId: string }>();

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["architecture", repoId],
    queryFn: () => getArchitecture(repoId!),
    enabled: Boolean(repoId),
  });

  if (isLoading) {
    return <p className="text-sm text-line-dim">Loading architecture&hellip;</p>;
  }

  if (isError) {
    return (
      <p className="text-sm text-danger">
        {error instanceof ApiError ? error.message : "Failed to load architecture."}
      </p>
    );
  }

  if (!data) return null;

  return (
    <div className="space-y-10">
      <div className="flex divide-x divide-line-faint border-y border-line-faint">
        <Reading label="Files" value={data.statistics.file_count} />
        <Reading label="Symbols" value={data.statistics.symbol_count} />
        <Reading label="Call edges" value={data.statistics.call_edges} />
      </div>

      {data.languages.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {data.languages.map((lang) => (
            <span key={lang} className="border border-line-faint px-2 py-0.5 font-mono text-xs text-line-dim">
              {lang}
            </span>
          ))}
        </div>
      )}

      <section>
        <h2 className="text-sm text-line-dim">Entrypoints</h2>
        {data.entrypoints.length === 0 ? (
          <p className="mt-2 text-sm text-line-faint">None detected.</p>
        ) : (
          <ul className="mt-2 divide-y divide-line-faint border-t border-line-faint">
            {data.entrypoints.map((ep, i) => (
              <li
                key={`${ep.file_path}-${ep.name}-${i}`}
                className="flex flex-wrap items-center gap-2 py-2"
              >
                <span className="font-mono text-sm text-line">{ep.name}</span>
                <span className="text-xs text-line-dim">{ep.type}</span>
                <CopyableLocation filePath={ep.file_path} />
              </li>
            ))}
          </ul>
        )}
      </section>

      <section>
        <h2 className="text-sm text-line-dim">Components ({data.components.length})</h2>
        {data.components.length === 0 ? (
          <p className="mt-2 text-sm text-line-faint">None detected.</p>
        ) : (
          <ul className="mt-2 divide-y divide-line-faint border-t border-line-faint">
            {data.components.map((c, i) => (
              <li
                key={`${c.file_path}-${c.name}-${i}`}
                className="flex flex-wrap items-center gap-2 py-2"
              >
                <span className="font-mono text-sm text-line">{c.name}</span>
                <span className="text-xs text-line-dim">
                  {c.type} &middot; {c.language}
                </span>
                <CopyableLocation filePath={c.file_path} startLine={c.start_line} endLine={c.end_line} />
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}

function Reading({ label, value }: { label: string; value: number }) {
  return (
    <div className="flex-1 px-4 py-3 first:pl-0">
      <div className="font-mono text-2xl text-line">{value}</div>
      <div className="text-xs text-line-dim">{label}</div>
    </div>
  );
}
