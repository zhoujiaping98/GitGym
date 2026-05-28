import type { RepoChangeEntry, RepoChangeGroups } from "../types";

const MAX_VISIBLE_REPO_CHANGES = 3;

export type SummarizedRepoChangeGroup = {
  title: "Staged" | "Unstaged" | "Untracked";
  count: number;
  all: RepoChangeEntry[];
  visible: RepoChangeEntry[];
  hiddenCount: number;
};

export type SummarizedFallbackRows = {
  all: string[];
  visible: string[];
  hiddenCount: number;
};

export type SummarizedRepoChanges = {
  groups: SummarizedRepoChangeGroup[];
  fallback: SummarizedFallbackRows;
};

function summarizeEntries(entries: RepoChangeEntry[]) {
  return {
    visible: entries.slice(0, MAX_VISIBLE_REPO_CHANGES),
    hiddenCount: Math.max(entries.length - MAX_VISIBLE_REPO_CHANGES, 0),
  };
}

function summarizeFallback(lines: string[]): SummarizedFallbackRows {
  return {
    all: lines,
    visible: lines.slice(0, MAX_VISIBLE_REPO_CHANGES),
    hiddenCount: Math.max(lines.length - MAX_VISIBLE_REPO_CHANGES, 0),
  };
}

export function summarizeRepoChanges(groups: RepoChangeGroups): SummarizedRepoChanges {
  const orderedGroups: Array<{
    title: SummarizedRepoChangeGroup["title"];
    entries: RepoChangeEntry[];
  }> = [
    { title: "Staged", entries: groups.staged },
    { title: "Unstaged", entries: groups.unstaged },
    { title: "Untracked", entries: groups.untracked },
  ];

  return {
    groups: orderedGroups
      .filter((group) => group.entries.length > 0)
      .map((group) => {
        const summarized = summarizeEntries(group.entries);
        return {
          title: group.title,
          count: group.entries.length,
          all: group.entries,
          visible: summarized.visible,
          hiddenCount: summarized.hiddenCount,
        };
      }),
    fallback: summarizeFallback(groups.fallback),
  };
}
