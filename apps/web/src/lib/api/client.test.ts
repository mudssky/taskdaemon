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
    } satisfies Partial<ApiClientError>);
  });

  it("calls trigger and cancel task endpoints", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      if (String(input).endsWith("/trigger")) {
        return new Response(
          JSON.stringify({ id: 3, status: "success", exitCode: 0 }),
          { status: 200 },
        );
      }
      return new Response(null, { status: 204 });
    });
    vi.stubGlobal("fetch", fetchMock);

    await expect(apiClient.triggerTask(7)).resolves.toMatchObject({
      id: 3,
      status: "success",
    });
    await expect(apiClient.cancelTask(7)).resolves.toBeUndefined();
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
  });

  it("calls auth status and initialize endpoints", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      if (String(input).endsWith("/status")) {
        return new Response(
          JSON.stringify({ initialized: false, authenticated: false }),
          { status: 200 },
        );
      }
      return new Response(
        JSON.stringify({
          adminId: 7,
          username: "admin",
          csrfToken: "csrf-token",
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
  });
});
