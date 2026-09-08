// Backend file paths look like "..\data\repos\<repoID>\internal\watcher\fs_watcher.go"
// (the clone directory's own on-disk location, not a clean repo-relative
// path) - these helpers strip everything up to and including the repoID
// segment, splitting on both \ and / since paths come from a Windows host.

export function repoRelativePath(filePath: string, repoId: string): string {
  const segments = filePath.split(/[\\/]/).filter(Boolean);
  const idx = segments.indexOf(repoId);
  const rel = idx >= 0 ? segments.slice(idx + 1) : segments;
  return rel.join("/");
}

export function topLevelDir(filePath: string, repoId: string): string {
  const rel = repoRelativePath(filePath, repoId).split("/");
  return rel.length > 1 ? rel[0] : "(root)";
}
