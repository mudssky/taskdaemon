import { describe, expect, it } from "vitest";
import {
  type CapabilityState,
  DesktopCapability,
  mapUnavailableReason,
} from "./capabilities";

describe("capabilities contract", () => {
  it("exposes named constants including Environment sample", () => {
    expect(DesktopCapability.Environment).toBe("desktop.environment");
    expect(DesktopCapability.Notification).toBe("desktop.notification");
    expect(DesktopCapability.Tray).toBe("desktop.tray");
    expect(DesktopCapability.Autostart).toBe("desktop.autostart");
    expect(DesktopCapability.WindowState).toBe("desktop.window-state");
  });

  it("maps unknown unavailable reasons to error", () => {
    expect(mapUnavailableReason("platform", false)).toBe("platform");
    expect(mapUnavailableReason("permission", false)).toBe("permission");
    expect(mapUnavailableReason("weird", false)).toBe("error");
    expect(mapUnavailableReason("platform", true)).toBeNull();
  });

  it("keeps invoke only on available branch at the type level", () => {
    function readInvoke(
      state: CapabilityState<string>,
    ): ((input?: unknown) => Promise<unknown>) | undefined {
      if (state.status === "available") {
        return state.invoke;
      }
      return undefined;
    }

    const unavailable: CapabilityState<string> = {
      status: "unavailable",
      reason: "not-desktop",
      message: "web",
    };
    const available: CapabilityState<string> = {
      status: "available",
      invoke: async () => ({ ok: true, data: "x" }),
    };

    expect(readInvoke(unavailable)).toBeUndefined();
    expect(typeof readInvoke(available)).toBe("function");
  });
});
