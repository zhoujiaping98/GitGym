import { describe, expect, it } from "vitest";
import type { RepoStateSnapshot } from "../types";
import { repoOutcomeCopy } from "./repoOutcome";

function snapshot(overrides: Partial<RepoStateSnapshot> = {}): RepoStateSnapshot {
  return {
    branch: "main",
    headCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    dirty: false,
    changedFiles: [],
    capturedAt: "2026-05-23T04:00:00.000Z",
    ...overrides,
  };
}

describe("repoOutcomeCopy", () => {
  it("reports when the working tree becomes dirty", () => {
    expect(
      repoOutcomeCopy(
        snapshot({ dirty: false, changedFiles: [] }),
        snapshot({ dirty: true, changedFiles: ["M notes.txt"] }),
      ),
    ).toBe("Working tree became dirty.");
  });

  it("reports when the working tree becomes clean", () => {
    expect(
      repoOutcomeCopy(
        snapshot({ dirty: true, changedFiles: ["M notes.txt"] }),
        snapshot({ dirty: false, changedFiles: [] }),
      ),
    ).toBe("Working tree is now clean.");
  });

  it("reports changed-file count deltas when dirty state does not flip", () => {
    expect(
      repoOutcomeCopy(
        snapshot({ dirty: true, changedFiles: ["M one.txt"] }),
        snapshot({ dirty: true, changedFiles: ["M one.txt", "M two.txt", "?? draft.md"] }),
      ),
    ).toBe("Changed files: 1 -> 3.");
  });

  it("reports branch changes ahead of count-only changes", () => {
    expect(
      repoOutcomeCopy(
        snapshot({ branch: "main", dirty: true, changedFiles: ["M one.txt"] }),
        snapshot({ branch: "feature/demo", dirty: true, changedFiles: ["M one.txt"] }),
      ),
    ).toBe("Branch changed: main -> feature/demo.");
  });

  it("reports head changes when branch and change count stay the same", () => {
    expect(
      repoOutcomeCopy(
        snapshot({ headCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", dirty: false }),
        snapshot({ headCommit: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", dirty: false }),
      ),
    ).toBe("HEAD changed.");
  });

  it("returns null when the snapshots are meaningfully unchanged", () => {
    expect(repoOutcomeCopy(snapshot(), snapshot())).toBeNull();
  });
});
