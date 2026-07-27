import { describe, expect, it } from "vitest";
import {
  hasNotificationFilters,
  parseNotificationSearch,
  toNotificationListParams,
} from "./notification.schema";

describe("parseNotificationSearch", () => {
  it("applies defaults for empty search", () => {
    expect(parseNotificationSearch({})).toEqual({
      page: 1,
      pageSize: 20,
      read: "all",
      severity: "all",
    });
  });

  it("parses valid filters", () => {
    expect(
      parseNotificationSearch({
        page: "3",
        pageSize: "10",
        read: "unread",
        severity: "error",
      }),
    ).toEqual({
      page: 3,
      pageSize: 10,
      read: "unread",
      severity: "error",
    });
  });

  it("falls back for illegal values without throwing", () => {
    expect(
      parseNotificationSearch({
        page: "0",
        pageSize: "9999",
        read: "maybe",
        severity: "fatal",
      }),
    ).toEqual({
      page: 1,
      pageSize: 20,
      read: "all",
      severity: "all",
    });
  });
});

describe("toNotificationListParams", () => {
  it("maps read filter to API boolean", () => {
    expect(
      toNotificationListParams({
        page: 2,
        pageSize: 10,
        read: "unread",
        severity: "warning",
      }),
    ).toEqual({
      page: 2,
      pageSize: 10,
      read: false,
      severity: "warning",
    });
    expect(
      toNotificationListParams({
        page: 1,
        pageSize: 20,
        read: "read",
        severity: "all",
      }),
    ).toEqual({
      page: 1,
      pageSize: 20,
      read: true,
    });
  });
});

describe("hasNotificationFilters", () => {
  it("detects active filters", () => {
    expect(
      hasNotificationFilters({
        page: 1,
        pageSize: 20,
        read: "all",
        severity: "all",
      }),
    ).toBe(false);
    expect(
      hasNotificationFilters({
        page: 1,
        pageSize: 20,
        read: "unread",
        severity: "all",
      }),
    ).toBe(true);
  });
});
