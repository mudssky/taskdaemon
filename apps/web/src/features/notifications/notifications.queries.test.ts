import { describe, expect, it } from "vitest";
import {
  NOTIFICATION_UNREAD_REFETCH_INTERVAL_MS,
  notificationKeys,
  resolveUnreadRefetchInterval,
} from "./notifications.queries";

describe("notificationKeys", () => {
  it("keeps stable prefixes for cache invalidation", () => {
    expect(notificationKeys.all).toEqual(["notifications"]);
    expect(notificationKeys.unreadCount[0]).toBe("notifications");
    expect(notificationKeys.preview[0]).toBe("notifications");
    expect(
      notificationKeys.list({ page: 1, pageSize: 20, read: false })[0],
    ).toBe("notifications");
  });
});

describe("resolveUnreadRefetchInterval", () => {
  it("polls only while page is visible", () => {
    expect(resolveUnreadRefetchInterval(true)).toBe(
      NOTIFICATION_UNREAD_REFETCH_INTERVAL_MS,
    );
    expect(resolveUnreadRefetchInterval(false)).toBe(false);
  });
});
