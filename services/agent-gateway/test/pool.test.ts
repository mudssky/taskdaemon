import {
  OMP_CLI_CAPABILITIES,
  PI_CLI_CAPABILITIES,
  PI_SDK_CAPABILITIES,
} from "@taskdaemon/agent-protocol";
import { describe, expect, it } from "vitest";
import { derivePoolPolicies, derivePoolPolicy } from "../src/runtime/pool.js";

describe("pool policy from coldStartCost", () => {
  it("CLI high → warmup + minIdle + long ttl", () => {
    const pi = derivePoolPolicy("pi", PI_CLI_CAPABILITIES, {
      hostMemoryGiB: 16,
    });
    expect(pi.warmup).toBe(true);
    expect(pi.minIdle).toBeGreaterThanOrEqual(1);
    expect(pi.idleTtlMs).toBeGreaterThanOrEqual(5 * 60_000);
    expect(pi.maxConcurrency).toBeLessThanOrEqual(8);
  });

  it("OMP heavy gets lower maxConcurrency than Pi light", () => {
    const policies = derivePoolPolicies(
      [
        { id: "pi", capabilities: PI_CLI_CAPABILITIES },
        { id: "omp", capabilities: OMP_CLI_CAPABILITIES },
      ],
      { hostMemoryGiB: 16 },
    );
    const pi = policies.find((p) => p.runtimeId === "pi");
    const omp = policies.find((p) => p.runtimeId === "omp");
    expect(pi).toBeDefined();
    expect(omp).toBeDefined();
    expect(omp?.maxConcurrency).toBeLessThanOrEqual(pi?.maxConcurrency ?? 0);
    expect(omp?.maxConcurrency).toBeLessThanOrEqual(3);
  });

  it("SDK low → no required warmup", () => {
    const sdk = derivePoolPolicy("pi-sdk", PI_SDK_CAPABILITIES);
    expect(sdk.coldStartCost).toBe("low");
    expect(sdk.warmup).toBe(false);
    expect(sdk.minIdle).toBe(0);
  });
});
