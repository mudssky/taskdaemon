import type { RuntimeCapabilities, Thread } from "@taskdaemon/agent-protocol";
import {
  modelSwitchControl,
  shouldShowWorkspaceUi,
  steeringControl,
  summarizeCapabilities,
} from "./capability-ui";

type CapabilityBarProps = {
  thread: Thread | undefined;
  capabilities: RuntimeCapabilities | undefined;
  model: string;
  onModelChange: (model: string) => void;
};

/**
 * 顶栏：能力摘要 + 条件模型选择 + workspace 提示。
 *
 * 参数:
 *   - props.thread / capabilities / model 绑定。
 *
 * 返回值:
 *   - 顶栏 React 节点。
 */
export function CapabilityBar({
  thread,
  capabilities,
  model,
  onModelChange,
}: CapabilityBarProps) {
  const modelCtl = modelSwitchControl(capabilities);
  const steerCtl = steeringControl(capabilities);
  const showWorkspace = shouldShowWorkspaceUi(thread, capabilities);

  return (
    <div className="flex flex-wrap items-center gap-3 border-b border-border bg-card px-4 py-2 text-xs text-muted-foreground">
      <span className="font-medium text-foreground">
        {thread
          ? String(thread.metadata?.title ?? thread.threadId)
          : "未选择会话"}
      </span>
      {thread && capabilities ? (
        <span
          className="truncate"
          title={summarizeCapabilities(capabilities, thread.runtimeId)}
        >
          {summarizeCapabilities(capabilities, thread.runtimeId)}
        </span>
      ) : null}

      {modelCtl.visible ? (
        <label className="ml-auto flex items-center gap-1">
          <span>模型</span>
          <select
            className="rounded border border-input bg-background px-2 py-1 text-foreground disabled:opacity-50"
            disabled={!modelCtl.enabled}
            value={model}
            onChange={(event) => onModelChange(event.target.value)}
            title={modelCtl.reason}
          >
            <option value={model || "default"}>{model || "default"}</option>
            <option value="mock-model">mock-model</option>
            <option value="mock-model-fast">mock-model-fast</option>
          </select>
          {!modelCtl.enabled && modelCtl.reason ? (
            <span className="text-warning-foreground">({modelCtl.reason})</span>
          ) : null}
        </label>
      ) : (
        <span className="ml-auto text-muted-foreground">
          模型选择不可用
          {modelCtl.reason ? `：${modelCtl.reason}` : ""}
        </span>
      )}

      {!steerCtl.visible ? (
        <span title={steerCtl.reason}>无 steering</span>
      ) : (
        <span className="text-success-foreground">steering 可用</span>
      )}

      {showWorkspace ? (
        <span className="rounded bg-muted px-2 py-0.5">
          workspace: {thread?.workspaceRoot ?? "(bound)"}
        </span>
      ) : (
        <span
          className="rounded bg-muted px-2 py-0.5"
          title="workspaceBinding=false"
        >
          无工作区
        </span>
      )}
    </div>
  );
}
