import { describe, expect, it } from "vitest";
import { FakeTransport, RpcClient } from "../src/runtime/rpc-client.js";

describe("RpcClient framing via FakeTransport", () => {
  it("accepts sticky multi-frame push", async () => {
    const t = new FakeTransport();
    const client = new RpcClient(t);
    const seen: string[] = [];
    client.onMessage((m) => {
      if (m.type) seen.push(String(m.type));
    });
    // 粘包
    t.push(
      `${JSON.stringify({ type: "a" })}\n${JSON.stringify({ type: "b" })}\n`,
    );
    expect(seen).toEqual(["a", "b"]);
    await client.close();
  });

  it("request resolves on response id", async () => {
    const t = new FakeTransport();
    t.setAutoHandler((req) => [
      {
        id: req.id,
        type: "response",
        command: req.type,
        success: true,
        data: { ok: true },
      },
    ]);
    const client = new RpcClient(t);
    const resp = await client.request("get_state");
    expect(resp.success).toBe(true);
    expect(resp.data).toEqual({ ok: true });
    await client.close();
  });

  it("process exit rejects pending and emits exit", async () => {
    const t = new FakeTransport();
    const client = new RpcClient(t, { requestTimeoutMs: 5_000 });
    let exited = false;
    client.onExit(() => {
      exited = true;
    });
    const pending = client.request("slow");
    t.exit(1);
    await expect(pending).rejects.toMatchObject({
      code: "AGENT_RUNTIME_UNAVAILABLE",
    });
    expect(exited).toBe(true);
    await client.close();
  });

  it("invalid JSON line does not kill client", async () => {
    const t = new FakeTransport();
    t.setAutoHandler((req) => [
      {
        id: req.id,
        type: "response",
        command: req.type,
        success: true,
      },
    ]);
    const client = new RpcClient(t);
    t.push("%%%not-json%%%\n");
    const resp = await client.request("get_state");
    expect(resp.success).toBe(true);
    await client.close();
  });
});
