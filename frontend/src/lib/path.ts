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

// For places that only have a raw backend path and no repoId at hand (the
// shared CopyableLocation chip): strips the clone directory prefix
// ("../data/repos/<repoID>/") so the server's on-disk layout never reaches the UI.
const CLONE_DIR_PREFIX = /^(?:\.{1,2}[\\/])*data[\\/]repos[\\/][^\\/]+[\\/]/;

export function displayPath(filePath: string): string {
  return filePath.replace(CLONE_DIR_PREFIX, "").replace(/\\/g, "/");
}

export function topLevelDir(filePath: string, repoId: string): string {
  const rel = repoRelativePath(filePath, repoId).split("/");
  return rel.length > 1 ? rel[0] : "(root)";
}
