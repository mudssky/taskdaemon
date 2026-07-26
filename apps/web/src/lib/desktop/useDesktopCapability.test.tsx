import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { PropsWithChildren, ReactElement } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { DesktopCapability } from "./capabilities";
import { useDesktopCapability } from "./useDesktopCapability";

const listMock = vi.fn();
const invokeMock = vi.fn();

vi.mock("./bridge", () => ({
  listDesktopCapabilities: () => listMock(),
  invokeDesktopCapability: (...args: unknown[]) => invokeMock(...args),
}));

vi.mock("./platform", () => ({
  isDesktopRuntime: vi.fn(),
}));

import { isDesktopRuntime } from "./platform";

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: 0 },
    },
  });
  return function Wrapper({ children }: PropsWithChildren): ReactElement {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  };
}

describe("useDesktopCapability", () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it("returns not-desktop without requesting catalog", async () => {
    vi.mocked(isDesktopRuntime).mockReturnValue(false);
    const { result } = renderHook(
      () => useDesktopCapability(DesktopCapability.Environment),
      { wrapper: createWrapper() },
    );

    expect(result.current).toMatchObject({
      status: "unavailable",
      reason: "not-desktop",
    });
    expect(listMock).not.toHaveBeenCalled();
  });

  it("returns checking while catalog loads", () => {
    vi.mocked(isDesktopRuntime).mockReturnValue(true);
    listMock.mockReturnValue(new Promise(() => {}));

    const { result } = renderHook(
      () => useDesktopCapability(DesktopCapability.Environment),
      { wrapper: createWrapper() },
    );

    expect(result.current.status).toBe("checking");
  });

  it("returns available with invoke when catalog hits", async () => {
    vi.mocked(isDesktopRuntime).mockReturnValue(true);
    listMock.mockResolvedValue([
      { name: DesktopCapability.Environment, available: true },
    ]);
    invokeMock.mockResolvedValue({
      ok: true,
      data: { platform: "darwin", arch: "arm64" },
    });

    const { result } = renderHook(
      () =>
        useDesktopCapability<{ platform: string }>(
          DesktopCapability.Environment,
        ),
      { wrapper: createWrapper() },
    );

    await waitFor(() => {
      expect(result.current.status).toBe("available");
    });

    if (result.current.status !== "available") {
      throw new Error("expected available");
    }
    const outcome = await result.current.invoke();
    expect(outcome).toEqual({
      ok: true,
      data: { platform: "darwin", arch: "arm64" },
    });
    expect(invokeMock).toHaveBeenCalledWith(
      DesktopCapability.Environment,
      undefined,
    );
  });

  it("returns structured invoke failure without throwing", async () => {
    vi.mocked(isDesktopRuntime).mockReturnValue(true);
    listMock.mockResolvedValue([
      { name: DesktopCapability.Environment, available: true },
    ]);
    invokeMock.mockResolvedValue({
      ok: false,
      code: "DESKTOP_INVOKE_FAILED",
      message: "boom",
    });

    const { result } = renderHook(
      () => useDesktopCapability(DesktopCapability.Environment),
      { wrapper: createWrapper() },
    );

    await waitFor(() => {
      expect(result.current.status).toBe("available");
    });
    if (result.current.status !== "available") {
      throw new Error("expected available");
    }
    await expect(result.current.invoke()).resolves.toEqual({
      ok: false,
      code: "DESKTOP_INVOKE_FAILED",
      message: "boom",
    });
  });

  it("does not expose invoke on unavailable branch (runtime shape)", async () => {
    vi.mocked(isDesktopRuntime).mockReturnValue(false);
    const { result } = renderHook(
      () => useDesktopCapability(DesktopCapability.Environment),
      { wrapper: createWrapper() },
    );
    expect(result.current.status).toBe("unavailable");
    expect(
      "invoke" in result.current
        ? (result.current as { invoke?: unknown }).invoke
        : undefined,
    ).toBeUndefined();
  });
});
