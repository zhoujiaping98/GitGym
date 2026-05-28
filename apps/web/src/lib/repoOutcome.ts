import type { RepoStateSnapshot } from "../types";

export function repoOutcomeCopy(previous: RepoStateSnapshot, current: RepoStateSnapshot) {
  if (!previous.dirty && current.dirty) {
    return "Working tree became dirty.";
  }

  if (previous.dirty && !current.dirty) {
    return "Working tree is now clean.";
  }

  if (previous.branch !== current.branch) {
    return `Branch changed: ${previous.branch} -> ${current.branch}.`;
  }

  if (previous.changedFiles.length !== current.changedFiles.length) {
    return `Changed files: ${previous.changedFiles.length} -> ${current.changedFiles.length}.`;
  }

  if (previous.headCommit !== current.headCommit) {
    return "HEAD changed.";
  }

  return null;
}
