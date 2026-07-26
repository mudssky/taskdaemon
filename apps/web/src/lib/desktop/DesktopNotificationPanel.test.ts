import { describe, expect, it } from "vitest";
import { DesktopCapability } from "./capabilities";

describe("DesktopCapability.Notification", () => {
  it("keeps C-5 notification capability name stable", () => {
    expect(DesktopCapability.Notification).toBe("desktop.notification");
  });
});
