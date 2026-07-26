import {
  OMP_CLI_CAPABILITIES,
  PI_CLI_CAPABILITIES,
} from "@taskdaemon/agent-protocol";
import { describe, expect, it } from "vitest";
import {
  AgentRuntimeError,
  createDefaultRuntimeRegistry,
  createOmpAdapter,
  createPiAdapter,
  createRuntimeRegistry,
} from "../src/runtime/index.js";
import { FakeTransport } from "../src/runtime/rpc-client.js";

function readyTransport(): FakeTransport {
  const t = new FakeTransport();
  t.setAutoHandler((req) => {
    if (req.type === "get_state") {
      return [
        {
          id: req.id,
          type: "response",
          command: "get_state",
          success: true,
          data: {
            sessionId: "rt-1",
            isStreaming: false,
            messageCount: 0,
            model: { id: "m", headers: { Authorization: "Bearer x" } },
          },
        },
      ];
    }
    return [
      {
        id: req.id,
        type: "response",
        command: String(req.type),
        success: true,
      },
    ];
  });
  return t;
}

describe("RuntimeRegistry", () => {
  it("registers by id and resolves without gateway knowing concrete types", () => {
    const registry = createRuntimeRegistry();
    const pi = createPiAdapter({
      transportFactory: async () => readyTransport(),
    });
    const omp = createOmpAdapter({
      transportFactory: async () => readyTransport(),
    });
    registry.register(pi);
    registry.register(omp);
    expect(registry.listIds()).toEqual(["omp", "pi"]);
    expect(registry.resolve("pi").id).toBe("pi");
    expect(registry.resolve("omp").capabilities().memoryClass).toBe("heavy");
  });

  it("adding adapter does not require changing resolve call sites", () => {
    const registry = createRuntimeRegistry();
    registry.register(
      createPiAdapter({ transportFactory: async () => readyTransport() }),
    );
    // 新 adapter 仅 register
    registry.register(
      createOmpAdapter({ transportFactory: async () => readyTransport() }),
    );
    for (const id of registry.listIds()) {
      const rt = registry.resolve(id);
      expect(typeof rt.createSession).toBe("function");
      expect(rt.capabilities().coldStartCost).toBeDefined();
    }
  });

  it("assertOptionalMethod rejects unsupported methods", () => {
    const registry = createRuntimeRegistry();
    // 伪造无 steering 的 runtime 通过 capabilitiesOverride
    const limited = createPiAdapter({
      transportFactory: async () => readyTransport(),
      capabilitiesOverride: {
        ...PI_CLI_CAPABILITIES,
        steering: false,
        followUp: false,
        modelSwitch: "none",
      },
    });
    // 去掉可选方法
    delete (limited as { steer?: unknown }).steer;
    delete (limited as { followUp?: unknown }).followUp;
    delete (limited as { setModel?: unknown }).setModel;
    registry.register(limited);
    const rt = registry.resolve("pi");
    expect(() => registry.assertOptionalMethod(rt, "steer")).toThrow(
      AgentRuntimeError,
    );
    try {
      registry.assertOptionalMethod(rt, "steer");
    } catch (e) {
      expect(e).toBeInstanceOf(AgentRuntimeError);
      expect((e as AgentRuntimeError).code).toBe(
        "AGENT_CAPABILITY_UNSUPPORTED",
      );
    }
  });

  it("createDefaultRuntimeRegistry catalogs coldStartCost for pooling", () => {
    const registry = createDefaultRuntimeRegistry({
      piTransportFactory: async () => readyTransport(),
      ompTransportFactory: async () => readyTransport(),
    });
    const catalog = registry.listCatalog();
    expect(catalog.find((c) => c.id === "pi")?.capabilities).toMatchObject(
      PI_CLI_CAPABILITIES,
    );
    expect(catalog.find((c) => c.id === "omp")?.capabilities).toMatchObject(
      OMP_CLI_CAPABILITIES,
    );
  });
});
