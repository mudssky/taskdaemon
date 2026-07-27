import { describe, expect, it } from "vitest";
import type { Notification } from "../../lib/api/types";
import {
  formatUnreadBadge,
  isNotificationRead,
  notificationEmptyMessage,
  resolveNotificationEmptyKind,
  resolveSubjectLink,
  severityLabel,
} from "./notification-format";

const baseNotification: Notification = {
  id: 1,
  eventId: "e1",
  name: "task.run.failed",
  severity: "error",
  title: "失败",
  occurredAt: "2026-07-27T10:00:00.000Z",
  createdAt: "2026-07-27T10:00:00.000Z",
};

describe("formatUnreadBadge", () => {
  it("hides zero and caps large counts", () => {
    expect(formatUnreadBadge(0)).toBeNull();
    expect(formatUnreadBadge(-1)).toBeNull();
    expect(formatUnreadBadge(7)).toBe("7");
    expect(formatUnreadBadge(99)).toBe("99");
    expect(formatUnreadBadge(100)).toBe("99+");
  });
});

describe("resolveNotificationEmptyKind", () => {
  it("distinguishes never vs filtered empty", () => {
    expect(resolveNotificationEmptyKind(0, false)).toBe("never");
    expect(resolveNotificationEmptyKind(0, true)).toBe("filtered");
    expect(resolveNotificationEmptyKind(3, false)).toBeNull();
    expect(notificationEmptyMessage("never")).toContain("还没有");
    expect(notificationEmptyMessage("filtered")).toContain("筛选");
  });
});

describe("resolveSubjectLink", () => {
  it("links existing task and disables deleted task", () => {
    const existing = resolveSubjectLink(
      {
        ...baseNotification,
        subjectKind: "task",
        subjectId: "12",
      },
      new Set([12]),
    );
    expect(existing).toEqual({
      available: true,
      target: { type: "task", taskId: 12 },
      label: "任务 #12",
    });

    const deleted = resolveSubjectLink(
      {
        ...baseNotification,
        subjectKind: "task",
        subjectId: "12",
      },
      new Set(),
    );
    expect(deleted).toEqual({
      available: false,
      label: "任务 #12",
      reason: "关联任务已删除",
    });
  });

  it("disables run link when related task is gone", () => {
    const link = resolveSubjectLink(
      {
        ...baseNotification,
        subjectKind: "run",
        subjectId: "88",
        detail: { taskId: 3 },
      },
      new Set(),
    );
    expect(link).toEqual({
      available: false,
      label: "运行 #88",
      reason: "关联任务已删除",
    });
  });

  it("links run history when task still exists", () => {
    const link = resolveSubjectLink(
      {
        ...baseNotification,
        subjectKind: "run",
        subjectId: "88",
        detail: { taskId: 3 },
      },
      new Set([3]),
    );
    expect(link).toEqual({
      available: true,
      target: { type: "runs" },
      label: "运行 #88",
    });
  });
});

describe("isNotificationRead", () => {
  it("uses readAt as source of truth", () => {
    expect(isNotificationRead(baseNotification)).toBe(false);
    expect(
      isNotificationRead({
        ...baseNotification,
        readAt: "2026-07-27T11:00:00.000Z",
      }),
    ).toBe(true);
  });
});

describe("severityLabel", () => {
  it("maps known severities", () => {
    expect(severityLabel("critical")).toBe("严重");
    expect(severityLabel("info")).toBe("信息");
  });
});
