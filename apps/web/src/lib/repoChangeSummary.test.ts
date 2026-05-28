import { describe, expect, it } from "vitest";
import type { RepoChangeGroups } from "../types";
import { summarizeRepoChanges } from "./repoChangeSummary";

function groups(overrides: Partial<RepoChangeGroups> = {}): RepoChangeGroups {
  return {
    staged: [],
    unstaged: [],
    untracked: [],
    fallback: [],
    ...overrides,
  };
}

describe("summarizeRepoChanges", () => {
  it("keeps all rows visible when a group has three or fewer entries", () => {
    const summary = summarizeRepoChanges(
      groups({
        staged: [
          { key: "1", bucket: "staged", label: "Modified", path: "a.txt" },
          { key: "2", bucket: "staged", label: "Added", path: "b.txt" },
          { key: "3", bucket: "staged", label: "Deleted", path: "c.txt" },
        ],
      }),
    );

    expect(summary.groups[0].title).toBe("Staged");
    expect(summary.groups[0].count).toBe(3);
    expect(summary.groups[0].visible).toHaveLength(3);
    expect(summary.groups[0].hiddenCount).toBe(0);
  });

  it("caps visible rows at three and reports the hidden remainder", () => {
    const summary = summarizeRepoChanges(
      groups({
        unstaged: [
          { key: "1", bucket: "unstaged", label: "Modified", path: "a.txt" },
          { key: "2", bucket: "unstaged", label: "Modified", path: "b.txt" },
          { key: "3", bucket: "unstaged", label: "Modified", path: "c.txt" },
          { key: "4", bucket: "unstaged", label: "Modified", path: "d.txt" },
        ],
      }),
    );

    expect(summary.groups[0].title).toBe("Unstaged");
    expect(summary.groups[0].count).toBe(4);
    expect(summary.groups[0].visible.map((entry) => entry.path)).toEqual([
      "a.txt",
      "b.txt",
      "c.txt",
    ]);
    expect(summary.groups[0].hiddenCount).toBe(1);
  });

  it("caps fallback rows with the same hidden remainder rule", () => {
    const summary = summarizeRepoChanges(
      groups({
        fallback: ["!! one", "!! two", "!! three", "!! four"],
      }),
    );

    expect(summary.fallback.visible).toEqual(["!! one", "!! two", "!! three"]);
    expect(summary.fallback.hiddenCount).toBe(1);
  });

  it("keeps the full group entries available alongside the collapsed visible rows", () => {
    const summary = summarizeRepoChanges(
      groups({
        unstaged: [
          { key: "1", bucket: "unstaged", label: "Modified", path: "a.txt" },
          { key: "2", bucket: "unstaged", label: "Modified", path: "b.txt" },
          { key: "3", bucket: "unstaged", label: "Modified", path: "c.txt" },
          { key: "4", bucket: "unstaged", label: "Modified", path: "d.txt" },
        ],
      }),
    );

    expect(summary.groups[0].visible.map((entry) => entry.path)).toEqual([
      "a.txt",
      "b.txt",
      "c.txt",
    ]);
    expect(summary.groups[0].all.map((entry) => entry.path)).toEqual([
      "a.txt",
      "b.txt",
      "c.txt",
      "d.txt",
    ]);
  });

  it("keeps the full fallback rows available alongside the collapsed visible rows", () => {
    const summary = summarizeRepoChanges(
      groups({
        fallback: ["!! one", "!! two", "!! three", "!! four"],
      }),
    );

    expect(summary.fallback.visible).toEqual(["!! one", "!! two", "!! three"]);
    expect(summary.fallback.all).toEqual(["!! one", "!! two", "!! three", "!! four"]);
  });
});
