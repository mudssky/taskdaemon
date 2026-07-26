import {
  OMP_CLI_CAPABILITIES,
  PI_CLI_CAPABILITIES,
  type RuntimeCapabilities,
  type Thread,
} from "@taskdaemon/agent-protocol";
import { describe, expect, it } from "vitest";
import {
  followUpControl,
  modelSwitchControl,
  shouldShowThinking,
  shouldShowWorkspaceUi,
  steeringControl,
  summarizeCapabilities,
  toolCallDetailMode,
} from "./capability-ui";

const degraded: RuntimeCapabilities = {
  ...OMP_CLI_CAPABILITIES,
  steering: false,
  followUp: false,
  thinking: false,
  toolCallDetail: "name-only",
  modelSwitch: "none",
  workspaceBinding: false,
};

const baseThread: Thread = {
  threadId: "t1",
  status: "idle",
  runtimeId: "pi",
  profile: "coding",
  workspaceBound: true,
  workspaceRoot: "/tmp",
  createdAt: "2026-07-27T00:00:00.000Z",
  updatedAt: "2026-07-27T00:00:00.000Z",
  tenantId: "dev",
  subject: "dev",
};

describe("capability-ui branches", () => {
  it("Pi 全能力：steering/model/thinking/workspace 均可用", () => {
    expect(steeringControl(PI_CLI_CAPABILITIES)).toEqual({
      visible: true,
      enabled: true,
    });
    expect(modelSwitchControl(PI_CLI_CAPABILITIES).visible).toBe(true);
    expect(modelSwitchControl(PI_CLI_CAPABILITIES).enabled).toBe(true);
    expect(shouldShowThinking(PI_CLI_CAPABILITIES)).toBe(true);
    expect(toolCallDetailMode(PI_CLI_CAPABILITIES)).toBe("full");
    expect(shouldShowWorkspaceUi(baseThread, PI_CLI_CAPABILITIES)).toBe(true);
    expect(followUpControl(PI_CLI_CAPABILITIES).visible).toBe(true);
  });

  it("降级 OMP mock：关闭入口且不伪造", () => {
    const steer = steeringControl(degraded);
    expect(steer.visible).toBe(false);
    expect(steer.reason).toMatch(/steering/);

    const model = modelSwitchControl(degraded);
    expect(model.visible).toBe(false);
    expect(model.reason).toMatch(/模型/);

    expect(shouldShowThinking(degraded)).toBe(false);
    expect(toolCallDetailMode(degraded)).toBe("name-only");
    expect(
      shouldShowWorkspaceUi(
        { ...baseThread, runtimeId: "omp", workspaceBound: false },
        degraded,
      ),
    ).toBe(false);
  });

  it("thread.workspaceBound=false 时隐藏工作区 UI", () => {
    expect(
      shouldShowWorkspaceUi(
        { ...baseThread, workspaceBound: false },
        PI_CLI_CAPABILITIES,
      ),
    ).toBe(false);
  });

  it("caps 未加载时保守处理", () => {
    expect(shouldShowThinking(undefined)).toBe(false);
    expect(toolCallDetailMode(undefined)).toBe("none");
    expect(shouldShowWorkspaceUi(baseThread, undefined)).toBe(false);
    expect(steeringControl(undefined).enabled).toBe(false);
  });

  it("summarize 不含硬编码分支逻辑，只摘要字段", () => {
    const text = summarizeCapabilities(degraded, "omp");
    expect(text).toContain("runtime=omp");
    expect(text).toContain("thinking=off");
    expect(text).toContain("tools=name-only");
  });
});
