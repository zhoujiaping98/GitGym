import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useCurrentSession } from "../hooks/useCurrentSession";
import * as api from "../lib/api";
import type { PracticeSession } from "../types";

const activeSession: PracticeSession = {
  id: 42,
  userId: 7,
  scenarioId: 9,
  templateId: 1,
  runnerRef: "runner-42",
  workspacePath: "/tmp/gitgym/session-42",
  status: "active",
  startedAt: "2026-05-16T10:00:00.000Z",
  expiresAt: "2026-05-16T12:00:00.000Z",
  lastActivityAt: "2026-05-16T10:05:00.000Z",
};

const mockFetchCurrentSession = vi.spyOn(api, "fetchCurrentSession");

beforeEach(() => {
  mockFetchCurrentSession.mockReset();
});

describe("useCurrentSession", () => {
  it("keeps the last known session mounted when refresh fails", async () => {
    mockFetchCurrentSession
      .mockResolvedValueOnce(activeSession)
      .mockRejectedValueOnce(new Error("api offline"));

    const { result } = renderHook(() => useCurrentSession());

    await waitFor(() => {
      expect(result.current.status).toBe("ready");
      expect(result.current.session).toEqual(activeSession);
    });

    await act(async () => {
      await expect(result.current.refresh()).rejects.toThrow("api offline");
    });

    await waitFor(() => {
      expect(result.current.status).toBe("ready");
    });

    expect(result.current.session).toEqual(activeSession);
    expect(result.current.error).toBe("api offline");
  });
});
