import { afterEach, describe, expect, it, vi } from "vitest";
import { type ApiClientError, apiClient } from "./client";

describe("apiClient", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("maps stable API errors", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(
        async () =>
          new Response(
            JSON.stringify({
              code: 1,
              msg: "nope",
              data: null,
              traceId: "trace-1",
              error: { code: "unauthorized", message: "nope" },
            }),
            { status: 401 },
          ),
      ),
    );

    await expect(apiClient.me()).rejects.toMatchObject({
      name: "ApiClientError",
      status: 401,
      code: "unauthorized",
      message: "nope",
      traceId: "trace-1",
    } satisfies Partial<ApiClientError>);
  });

  it("calls trigger, cancel and delete task endpoints", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      if (String(input).endsWith("/trigger")) {
        return new Response(
          JSON.stringify({
            code: 0,
            msg: "ok",
            data: { id: 3, status: "success", exitCode: 0 },
          }),
          { status: 200 },
        );
      }
      return new Response(JSON.stringify({ code: 0, msg: "ok", data: null }), {
        status: 200,
      });
    });
    vi.stubGlobal("fetch", fetchMock);

    await expect(apiClient.triggerTask(7)).resolves.toMatchObject({
      id: 3,
      status: "success",
    });
    await expect(apiClient.cancelTask(7)).resolves.toBeUndefined();
    await expect(apiClient.deleteTask(7)).resolves.toBeUndefined();
    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/tasks/7/trigger",
      expect.objectContaining({ method: "POST", credentials: "include" }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/tasks/7/cancel",
      expect.objectContaining({ method: "POST", credentials: "include" }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      3,
      "/api/tasks/7",
      expect.objectContaining({ method: "DELETE", credentials: "include" }),
    );
  });

  it("calls auth status, initialize and logout endpoints", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      if (String(input).endsWith("/status")) {
        return new Response(
          JSON.stringify({
            code: 0,
            msg: "ok",
            data: { initialized: false, authenticated: false },
          }),
          { status: 200 },
        );
      }
      if (String(input).endsWith("/logout")) {
        return new Response(
          JSON.stringify({ code: 0, msg: "ok", data: null }),
          { status: 200 },
        );
      }
      return new Response(
        JSON.stringify({
          code: 0,
          msg: "ok",
          data: {
            adminId: 7,
            username: "admin",
            csrfToken: "csrf-token",
          },
        }),
        { status: 201 },
      );
    });
    vi.stubGlobal("fetch", fetchMock);

    await expect(apiClient.authStatus()).resolves.toEqual({
      initialized: false,
      authenticated: false,
    });
    await expect(apiClient.initializeAdmin("admin", "secret")).resolves.toEqual(
      {
        adminId: 7,
        username: "admin",
        csrfToken: "csrf-token",
      },
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/auth/status",
      expect.objectContaining({ credentials: "include" }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/auth/init",
      expect.objectContaining({
        method: "POST",
        credentials: "include",
        body: JSON.stringify({ username: "admin", password: "secret" }),
      }),
    );
    await expect(apiClient.logout()).resolves.toBeUndefined();
    expect(fetchMock).toHaveBeenNthCalledWith(
      3,
      "/api/auth/logout",
      expect.objectContaining({ method: "POST", credentials: "include" }),
    );
  });

  it("calls audio history, replay and config endpoints", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      if (String(input).endsWith("/config/audio")) {
        return new Response(
          JSON.stringify({
            code: 0,
            msg: "ok",
            data: {
              autoplay: { enabled: true, target: "backend" },
              playback: { queueLimit: 20 },
              inbound: {
                tokenConfigured: true,
                maxBytes: 209715200,
                url: {
                  allowedSchemes: ["https"],
                  allowPrivateNetworks: false,
                  allowedHosts: [],
                  downloadTimeoutSeconds: 60,
                  maxRedirects: 3,
                },
              },
              history: { limit: 50 },
              ffmpeg: {
                pathConfigured: false,
                probePathConfigured: false,
                transcodeTimeoutSeconds: 120,
              },
              configuration: {
                runtimeEditable: ["autoplay"],
                restartRequired: ["ffmpeg.transcodeTimeoutSeconds"],
                fileOnly: [
                  "ffmpeg.path",
                  "ffmpeg.probePath",
                  "inbound.tokenHash",
                ],
              },
            },
          }),
          { status: 200 },
        );
      }
      if (String(input).endsWith("/replay")) {
        return new Response(
          JSON.stringify({
            code: 0,
            msg: "ok",
            data: { id: 9, sourceKind: "upload", status: "queued" },
          }),
          { status: 200 },
        );
      }
      return new Response(
        JSON.stringify({
          code: 0,
          msg: "ok",
          data: {
            records: [{ id: 9, sourceKind: "upload", status: "played" }],
          },
        }),
        { status: 200 },
      );
    });
    vi.stubGlobal("fetch", fetchMock);

    await expect(apiClient.listAudioHistory(25)).resolves.toMatchObject({
      records: [{ id: 9, status: "played" }],
    });
    await expect(apiClient.replayAudioRecord(9)).resolves.toMatchObject({
      id: 9,
      status: "queued",
    });
    await expect(apiClient.audioConfig()).resolves.toMatchObject({
      autoplay: { enabled: true, target: "backend" },
      inbound: { tokenConfigured: true },
    });
    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/audio/history?limit=25",
      expect.objectContaining({ credentials: "include" }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/audio/history/9/replay",
      expect.objectContaining({ method: "POST", credentials: "include" }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      3,
      "/api/config/audio",
      expect.objectContaining({ credentials: "include" }),
    );
  });

  it("puts audio config section and returns write response", async () => {
    const fetchMock = vi.fn(async () => {
      return new Response(
        JSON.stringify({
          code: 0,
          msg: "ok",
          data: {
            config: {
              autoplay: { enabled: false, target: "backend" },
              playback: { queueLimit: 8 },
              inbound: {
                tokenConfigured: true,
                maxBytes: 209715200,
                url: {
                  allowedSchemes: ["https"],
                  allowPrivateNetworks: false,
                  allowedHosts: [],
                  downloadTimeoutSeconds: 60,
                  maxRedirects: 3,
                },
              },
              history: { limit: 50 },
              ffmpeg: {
                pathConfigured: false,
                probePathConfigured: false,
                transcodeTimeoutSeconds: 120,
              },
              configuration: {
                runtimeEditable: ["autoplay", "playback", "inbound", "history"],
                restartRequired: ["ffmpeg.transcodeTimeoutSeconds"],
                fileOnly: [
                  "ffmpeg.path",
                  "ffmpeg.probePath",
                  "inbound.tokenHash",
                ],
              },
            },
            applied: ["audio.playback.queueLimit"],
            restartRequired: [],
            reload: {
              applied: ["audio.playback.queueLimit"],
              restartRequired: [],
              subsystems: [
                { name: "runtime", status: "ok" },
                { name: "audio", status: "ok" },
              ],
            },
          },
          traceId: "trace-config-write",
        }),
        { status: 200 },
      );
    });
    vi.stubGlobal("fetch", fetchMock);

    const result = await apiClient.putConfigSection("audio", {
      playback: { queueLimit: 8 },
      inbound: { token: "once-token" },
    });

    expect(result.applied).toEqual(["audio.playback.queueLimit"]);
    expect(result.config.inbound.tokenConfigured).toBe(true);
    expect(result.reload.subsystems).toEqual(
      expect.arrayContaining([
        { name: "runtime", status: "ok" },
        { name: "audio", status: "ok" },
      ]),
    );
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/config/audio",
      expect.objectContaining({
        method: "PUT",
        credentials: "include",
        body: JSON.stringify({
          playback: { queueLimit: 8 },
          inbound: { token: "once-token" },
        }),
      }),
    );
  });

  it("calls notification list, unread-count and mutation endpoints", async () => {
    const fetchMock = vi.fn(
      async (input: RequestInfo | URL, init?: RequestInit) => {
        const path = String(input);
        if (path.startsWith("/api/notifications/unread-count")) {
          return new Response(
            JSON.stringify({ code: 0, msg: "ok", data: { count: 3 } }),
            { status: 200 },
          );
        }
        if (path === "/api/notifications/1/read") {
          return new Response(
            JSON.stringify({
              code: 0,
              msg: "ok",
              data: { id: 1, title: "done", readAt: "2026-07-27T00:00:00Z" },
            }),
            { status: 200 },
          );
        }
        if (path === "/api/notifications/read" && init?.method === "POST") {
          return new Response(
            JSON.stringify({ code: 0, msg: "ok", data: { affected: 2 } }),
            { status: 200 },
          );
        }
        if (path === "/api/notifications/read-all") {
          return new Response(
            JSON.stringify({ code: 0, msg: "ok", data: { affected: 5 } }),
            { status: 200 },
          );
        }
        if (path === "/api/notifications/read" && init?.method === "DELETE") {
          return new Response(
            JSON.stringify({ code: 0, msg: "ok", data: { affected: 4 } }),
            { status: 200 },
          );
        }
        if (path === "/api/notifications/9" && init?.method === "DELETE") {
          return new Response(
            JSON.stringify({ code: 0, msg: "ok", data: null }),
            { status: 200 },
          );
        }
        return new Response(
          JSON.stringify({
            code: 0,
            msg: "ok",
            data: {
              notifications: [{ id: 1, title: "ok", severity: "info" }],
              total: 1,
              page: 1,
              pageSize: 20,
            },
          }),
          { status: 200 },
        );
      },
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      apiClient.listNotifications({
        page: 2,
        pageSize: 10,
        read: false,
        severity: "error",
      }),
    ).resolves.toMatchObject({
      total: 1,
      notifications: [{ id: 1 }],
    });
    await expect(apiClient.notificationUnreadCount()).resolves.toEqual({
      count: 3,
    });
    await expect(apiClient.markNotificationRead(1)).resolves.toMatchObject({
      id: 1,
    });
    await expect(apiClient.markNotificationsRead([1, 2])).resolves.toEqual({
      affected: 2,
    });
    await expect(apiClient.markAllNotificationsRead()).resolves.toEqual({
      affected: 5,
    });
    await expect(apiClient.deleteNotification(9)).resolves.toBeUndefined();
    await expect(apiClient.clearReadNotifications()).resolves.toEqual({
      affected: 4,
    });

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/notifications?page=2&pageSize=10&read=false&severity=error",
      expect.objectContaining({ credentials: "include" }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/notifications/unread-count",
      expect.objectContaining({ credentials: "include" }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      3,
      "/api/notifications/1/read",
      expect.objectContaining({ method: "POST", credentials: "include" }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      4,
      "/api/notifications/read",
      expect.objectContaining({ method: "POST", credentials: "include" }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      5,
      "/api/notifications/read-all",
      expect.objectContaining({ method: "POST", credentials: "include" }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      6,
      "/api/notifications/9",
      expect.objectContaining({ method: "DELETE", credentials: "include" }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      7,
      "/api/notifications/read",
      expect.objectContaining({ method: "DELETE", credentials: "include" }),
    );
  });
});
