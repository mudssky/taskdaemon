import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { apiClient } from "../../lib/api/client";
import type { TaskRun } from "../../lib/api/types";
import { RunHistoryTable } from "./RunHistoryTable";

vi.mock("../../lib/api/client", () => ({
  apiClient: {
    downloadRunLog: vi.fn(),
  },
  ApiClientError: class ApiClientError extends Error {
    constructor(
      public status: number,
      public code: string,
      message: string,
    ) {
      super(message);
    }
  },
}));

function baseRun(overrides: Partial<TaskRun> = {}): TaskRun {
  return {
    id: 1,
    trigger: "manual",
    status: "success",
    exitCode: 0,
    startedAt: "2026-07-27T00:00:00.000Z",
    finishedAt: "2026-07-27T00:00:01.000Z",
    durationMs: 1000,
    errorSummary: "",
    stdout: "ok",
    stderr: "",
    logArchiveStatus: "archived",
    ...overrides,
  };
}

describe("RunHistoryTable runlog download", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows download button for archived", () => {
    render(
      <RunHistoryTable
        runs={[baseRun({ logArchiveStatus: "archived" })]}
        isLoading={false}
      />,
    );
    expect(
      screen.getByRole("button", { name: "下载完整日志" }),
    ).toBeInTheDocument();
  });

  it("shows absent hint for old data", () => {
    render(
      <RunHistoryTable
        runs={[baseRun({ logArchiveStatus: "absent" })]}
        isLoading={false}
      />,
    );
    expect(screen.getByText("无归档（旧数据）")).toBeInTheDocument();
  });

  it("shows pruned hint", () => {
    render(
      <RunHistoryTable
        runs={[baseRun({ logArchiveStatus: "pruned" })]}
        isLoading={false}
      />,
    );
    expect(screen.getByText("已清理")).toBeInTheDocument();
  });

  it("triggers download for archived run", async () => {
    vi.mocked(apiClient.downloadRunLog).mockResolvedValue(new Blob(["log"]));
    const createObjectURL = vi.fn(() => "blob:mock");
    const revokeObjectURL = vi.fn();
    Object.defineProperty(URL, "createObjectURL", {
      value: createObjectURL,
      configurable: true,
    });
    Object.defineProperty(URL, "revokeObjectURL", {
      value: revokeObjectURL,
      configurable: true,
    });
    const click = vi.fn();
    const originalCreate = document.createElement.bind(document);
    vi.spyOn(document, "createElement").mockImplementation((tag: string) => {
      const el = originalCreate(tag);
      if (tag === "a") {
        Object.defineProperty(el, "click", { value: click });
      }
      return el;
    });

    render(
      <RunHistoryTable
        runs={[baseRun({ id: 9, logArchiveStatus: "archived" })]}
        isLoading={false}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: "下载完整日志" }));
    expect(apiClient.downloadRunLog).toHaveBeenCalledWith(9);
    expect(click).toHaveBeenCalled();
  });
});
