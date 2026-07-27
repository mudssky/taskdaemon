import { afterEach, describe, expect, it, vi } from "vitest";
import {
  invokeDesktopCapability,
  listDesktopCapabilities,
  WAILS_INVOKE_CAPABILITY,
  WAILS_LIST_CAPABILITIES,
} from "./bridge";

describe("bridge", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
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
});
