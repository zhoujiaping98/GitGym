import { describe, expect, it } from "vitest";
import { groupRepoChanges } from "./repoChanges";

describe("groupRepoChanges", () => {
  it("groups staged, unstaged, and untracked entries", () => {
    const grouped = groupRepoChanges([
      "M  staged-only.txt",
      " M unstaged-only.txt",
      "MM both.txt",
      "?? draft.md",
    ]);

    expect(grouped.staged).toEqual([
      {
        key: "staged:Modified:staged-only.txt",
        bucket: "staged",
        label: "Modified",
        path: "staged-only.txt",
      },
      {
        key: "staged:Modified:both.txt",
        bucket: "staged",
        label: "Modified",
        path: "both.txt",
      },
    ]);
    expect(grouped.unstaged).toEqual([
      {
        key: "unstaged:Modified:unstaged-only.txt",
        bucket: "unstaged",
        label: "Modified",
        path: "unstaged-only.txt",
      },
      {
        key: "unstaged:Modified:both.txt",
        bucket: "unstaged",
        label: "Modified",
        path: "both.txt",
      },
    ]);
    expect(grouped.untracked).toEqual([
      {
        key: "untracked:Untracked:draft.md",
        bucket: "untracked",
        label: "Untracked",
        path: "draft.md",
      },
    ]);
    expect(grouped.fallback).toEqual([]);
  });

  it("preserves rename text and falls back for unknown rows", () => {
    const grouped = groupRepoChanges(["R  old.txt -> new.txt", "!! ignored.tmp"]);

    expect(grouped.staged).toEqual([
      {
        key: "staged:Renamed:old.txt -> new.txt",
        bucket: "staged",
        label: "Renamed",
        path: "old.txt -> new.txt",
      },
    ]);
    expect(grouped.fallback).toEqual(["!! ignored.tmp"]);
  });

  it("treats unmerged rows as conflicted entries", () => {
    const grouped = groupRepoChanges(["UU notes.txt"]);

    expect(grouped.staged).toEqual([
      {
        key: "staged:Conflicted:notes.txt",
        bucket: "staged",
        label: "Conflicted",
        path: "notes.txt",
      },
    ]);
    expect(grouped.unstaged).toEqual([
      {
        key: "unstaged:Conflicted:notes.txt",
        bucket: "unstaged",
        label: "Conflicted",
        path: "notes.txt",
      },
    ]);
  });
});
