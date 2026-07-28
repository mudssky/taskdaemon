import { afterEach, describe, expect, it, vi } from "vitest";
import {
  DESKTOP_HTTP_CAPABILITIES,
  DESKTOP_HTTP_INVOKE,
  invokeDesktopCapability,
  listDesktopCapabilities,
  WAILS_INVOKE_CAPABILITY,
  WAILS_LIST_CAPABILITIES,
} from "./bridge";

describe("bridge", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
    const w = window as Window & { wails?: unknown; _wails?: unknown };
    delete w.wails;
    delete w._wails;
  });

  it("listDesktopCapabilities calls Wails ByName", async () => {
    const byName = vi
      .fn()
      .mockResolvedValue([{ name: "desktop.environment", available: true }]);
    (window as Window & { wails?: unknown; _wails?: unknown })._wails = {};
    (window as Window & { wails?: unknown }).wails = {
      Call: { ByName: byName },
    };

    await expect(listDesktopCapabilities()).resolves.toEqual([
      { name: "desktop.environment", available: true },
    ]);
    expect(byName).toHaveBeenCalledWith(WAILS_LIST_CAPABILITIES);
  });

  it("listDesktopCapabilities falls back to HTTP bridge", async () => {
    (window as Window & { _wails?: unknown })._wails = {};
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        data: [{ name: "desktop.environment", available: true }],
      }),
    });
    vi.stubGlobal("fetch", fetchMock);

    await expect(listDesktopCapabilities()).resolves.toEqual([
      { name: "desktop.environment", available: true },
    ]);
    expect(fetchMock).toHaveBeenCalledWith(
      DESKTOP_HTTP_CAPABILITIES,
      expect.objectContaining({ method: "GET", credentials: "include" }),
    );
  });

  it("invokeDesktopCapability returns structured failure without throw", async () => {
    const byName = vi.fn().mockRejectedValue(new Error("native boom"));
    (window as Window & { wails?: unknown; _wails?: unknown })._wails = {};
    (window as Window & { wails?: unknown }).wails = {
      Call: { ByName: byName },
    };

    await expect(
      invokeDesktopCapability("desktop.environment"),
    ).resolves.toEqual({
      ok: false,
      code: "DESKTOP_INVOKE_FAILED",
      message: "native boom",
    });
    expect(byName).toHaveBeenCalledWith(
      WAILS_INVOKE_CAPABILITY,
      "desktop.environment",
      "",
    );
  });

  it("invokeDesktopCapability maps ok result", async () => {
    const byName = vi.fn().mockResolvedValue({
      ok: true,
      data: { platform: "darwin" },
    });
    (window as Window & { wails?: unknown; _wails?: unknown })._wails = {};
    (window as Window & { wails?: unknown }).wails = {
      Call: { ByName: byName },
    };

    await expect(
      invokeDesktopCapability("desktop.environment", { x: 1 }),
    ).resolves.toEqual({
      ok: true,
      data: { platform: "darwin" },
    });
    expect(byName).toHaveBeenCalledWith(
      WAILS_INVOKE_CAPABILITY,
      "desktop.environment",
      JSON.stringify({ x: 1 }),
    );
  });

  it("invokeDesktopCapability falls back to HTTP bridge", async () => {
    (window as Window & { _wails?: unknown })._wails = {};
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        data: { ok: true, data: { platform: "windows" } },
      }),
    });
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      invokeDesktopCapability("desktop.environment", { x: 1 }),
    ).resolves.toEqual({
      ok: true,
      data: { platform: "windows" },
    });
    expect(fetchMock).toHaveBeenCalledWith(
      DESKTOP_HTTP_INVOKE,
      expect.objectContaining({
        method: "POST",
        credentials: "include",
      }),
    );
    const init = fetchMock.mock.calls[0]?.[1] as RequestInit;
    expect(JSON.parse(String(init.body))).toEqual({
      name: "desktop.environment",
      payloadJSON: JSON.stringify({ x: 1 }),
    });
  });
});
