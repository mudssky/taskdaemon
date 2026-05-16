import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryHistory, RouterProvider } from "@tanstack/react-router";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { PropsWithChildren } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { createAppRouter } from "./router";

const taskFixtures = [
  {
    id: 1,
    name: "backup",
    description: "nightly backup",
    enabled: true,
    cronExpression: "30 9 * * *",
    timezone: "Local",
    runnerType: "shell",
    runnerConfig: {},
    timeoutSeconds: 3600,
    overlapPolicy: "skip",
    running: false,
    createdAt: "2026-05-16T00:00:00Z",
    updatedAt: "2026-05-16T00:00:00Z",
  },
];

const runFixtures = [
  {
    id: 11,
    trigger: "cron",
    status: "success",
    exitCode: 0,
    startedAt: "2026-05-16T01:00:00Z",
    finishedAt: "2026-05-16T01:00:03Z",
    durationMs: 3000,
    errorSummary: "",
    stdout: "ok",
    stderr: "",
  },
];

function renderRouter(initialPath = "/") {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  const router = createAppRouter(
    createMemoryHistory({ initialEntries: [initialPath] }),
  );
  const wrapper = ({ children }: PropsWithChildren) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return render(<RouterProvider router={router} />, { wrapper });
}

describe("router", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("switches sidebar pages without stacking task and run views on home", async () => {
    const user = userEvent.setup();
    vi.stubGlobal("scrollTo", vi.fn());
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const path = String(input);
        if (path === "/api/auth/status") {
          return jsonResponse({
            initialized: true,
            authenticated: true,
            admin: { adminId: 1, username: "admin" },
          });
        }
        if (path === "/api/tasks") {
          return jsonResponse({ tasks: taskFixtures });
        }
        if (path === "/api/tasks/1/runs") {
          return jsonResponse({ runs: runFixtures });
        }
        return jsonResponse(null);
      }),
    );

    renderRouter();

    expect(
      await screen.findByRole("heading", { level: 1, name: "运行状态" }),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(
        screen.getByRole("heading", { name: "当前关注" }),
      ).toBeInTheDocument(),
    );
    expect(
      screen.queryByRole("heading", { name: "创建任务" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByLabelText("历史范围")).not.toBeInTheDocument();

    await user.click(screen.getByRole("link", { name: "任务" }));
    expect(
      await screen.findByRole("heading", { level: 1, name: "任务" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "任务列表" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "创建任务" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "当前关注" }),
    ).not.toBeInTheDocument();

    await user.click(screen.getByRole("link", { name: "执行历史" }));
    expect(
      await screen.findByRole("heading", { level: 1, name: "执行历史" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "历史范围" }),
    ).toBeInTheDocument();
    const runsPanel = screen
      .getByRole("heading", { level: 2, name: "执行历史" })
      .closest("section");
    expect(runsPanel).not.toBeNull();
    expect(
      within(runsPanel as HTMLElement).getByText("backup"),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "创建任务" }),
    ).not.toBeInTheDocument();
  });
});

function jsonResponse(data: unknown, status = 200) {
  return new Response(JSON.stringify({ code: 0, msg: "ok", data }), { status });
}
