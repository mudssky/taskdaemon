import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { PropsWithChildren } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AdminSetupPanel } from "./AdminSetupPanel";

function renderWithQueryClient(ui: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  const wrapper = ({ children }: PropsWithChildren) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return render(ui, { wrapper });
}

describe("AdminSetupPanel", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("blocks mismatched passwords and submits first admin credentials", async () => {
    const user = userEvent.setup();
    const fetchMock = vi.fn(
      async () =>
        new Response(
          JSON.stringify({
            adminId: 7,
            username: "admin",
            csrfToken: "csrf-token",
          }),
          { status: 201 },
        ),
    );
    vi.stubGlobal("fetch", fetchMock);
    renderWithQueryClient(<AdminSetupPanel />);

    await user.type(screen.getByLabelText("密码"), "secret");
    await user.type(screen.getByLabelText("确认密码"), "wrong");
    expect(screen.getByRole("button", { name: "创建管理员" })).toBeDisabled();
    expect(fetchMock).not.toHaveBeenCalled();

    await user.clear(screen.getByLabelText("确认密码"));
    await user.type(screen.getByLabelText("确认密码"), "secret");
    await user.click(screen.getByRole("button", { name: "创建管理员" }));

    await waitFor(() => expect(fetchMock).toHaveBeenCalledOnce());
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/auth/init",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ username: "admin", password: "secret" }),
      }),
    );
  });
});
