import { afterEach, describe, expect, it, vi } from "vitest";
import { getPlatformInfo, isDesktopRuntime } from "./platform";

type MarkerWindow = Window & {
  _wails?: unknown;
  wails?: {
    Call?: { ByName?: (...args: unknown[]) => Promise<unknown> };
  };
};

function host(): MarkerWindow {
  return window as MarkerWindow;
}

describe("platform", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    const w = host();
    delete w._wails;
    delete w.wails;
  });

  it("returns false without native markers", () => {
    expect(isDesktopRuntime()).toBe(false);
    expect(getPlatformInfo()).toEqual({
      runtime: "web",
      hasNativeBridge: false,
    });
  });

  it("returns true when native runtime marker is present", () => {
    host()._wails = {};
    expect(isDesktopRuntime()).toBe(true);
    expect(getPlatformInfo().runtime).toBe("desktop");
  });

  it("returns true when Call.ByName is present", () => {
    host().wails = {
      Call: { ByName: vi.fn() },
    };
    expect(isDesktopRuntime()).toBe(true);
  });
});
