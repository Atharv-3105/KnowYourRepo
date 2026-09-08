// A single instrument-panel-style stat readout - shared between
// Architecture and Overview, both of which render the same
// files/symbols/call-edges row from the same GET /architecture/:repoID
// statistics object.
export default function StatReading({ label, value }: { label: string; value: number }) {
  return (
    <div className="flex-1 px-4 py-3 first:pl-0">
      <div className="font-mono text-2xl text-ink">{value}</div>
      <div className="text-xs text-ink-dim">{label}</div>
    </div>
  );
}
