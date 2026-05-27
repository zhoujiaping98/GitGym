import type { RepoChangeBucket, RepoChangeEntry, RepoChangeGroups } from "../types";

function entry(bucket: RepoChangeBucket, label: string, path: string): RepoChangeEntry {
  return {
    key: `${bucket}:${label}:${path}`,
    bucket,
    label,
    path,
  };
}

function labelForCode(code: string, otherCode: string) {
  if (code === "U" || otherCode === "U") {
    return "Conflicted";
  }

  if (code === "M") {
    return "Modified";
  }
  if (code === "A") {
    return "Added";
  }
  if (code === "D") {
    return "Deleted";
  }
  if (code === "R") {
    return "Renamed";
  }
  if (code === "C") {
    return "Copied";
  }

  return null;
}

export function groupRepoChanges(lines: string[]): RepoChangeGroups {
  const groups: RepoChangeGroups = {
    staged: [],
    unstaged: [],
    untracked: [],
    fallback: [],
  };

  for (const line of lines) {
    if (line.startsWith("?? ")) {
      groups.untracked.push(entry("untracked", "Untracked", line.slice(3)));
      continue;
    }

    if (line.length < 4) {
      groups.fallback.push(line);
      continue;
    }

    const compactSingleColumn = line[1] === " " && line[2] !== " ";
    const stagedCode = line[0];
    const unstagedCode = compactSingleColumn ? " " : line[1];
    const path = compactSingleColumn ? line.slice(2) : line.slice(3);

    let parsed = false;
    const stagedLabel = stagedCode === " " ? null : labelForCode(stagedCode, unstagedCode);
    const unstagedLabel = unstagedCode === " " ? null : labelForCode(unstagedCode, stagedCode);

    if (stagedLabel) {
      groups.staged.push(entry("staged", stagedLabel, path));
      parsed = true;
    }
    if (unstagedLabel) {
      groups.unstaged.push(entry("unstaged", unstagedLabel, path));
      parsed = true;
    }

    if (!parsed) {
      groups.fallback.push(line);
    }
  }

  return groups;
}
