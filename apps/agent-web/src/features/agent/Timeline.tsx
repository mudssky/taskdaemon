import type {
  HitlDecision,
  RuntimeCapabilities,
} from "@taskdaemon/agent-protocol";
import { useState } from "react";
import { shouldShowThinking } from "./capability-ui";
import { runStatusLabel } from "./hitl";
import { splitMessageSegments } from "./message-format";
import type { StreamViewState, TimelineItem } from "./stream-reducer";

type TimelineProps = {
  state: StreamViewState;
  capabilities: RuntimeCapabilities | undefined;
  onToggleTool: (toolCallId: string) => void;
  onToggleThinking: (messageId: string) => void;
  onHitl: (
    requestId: string,
    decision: HitlDecision,
    modifiedText?: string,
  ) => void;
};

/**
 * 消息 + 工具 + HITL + steering 时间线。
 *
 * 参数:
 *   - props: 状态与回调。
 *
 * 返回值:
 *   - 时间线节点。
 */
export function Timeline({
  state,
  capabilities,
  onToggleTool,
  onToggleThinking,
  onHitl,
}: TimelineProps) {
  const showThinking = shouldShowThinking(capabilities);

  if (state.items.length === 0) {
    return (
      <div className="flex h-full items-center justify-center p-4 text-sm text-muted-foreground">
        暂无消息。发送第一条开始对话。
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-3 p-4">
      {state.status === "awaiting_human" ? (
        <div className="rounded-md border border-warning-border bg-warning px-3 py-2 text-sm font-medium text-warning-foreground">
          {runStatusLabel("awaiting_human")} —
          请在下方决策点操作，程序并未卡住。
        </div>
      ) : null}
      {state.items.map((item) => (
        <TimelineRow
          key={item.id}
          item={item}
          showThinking={showThinking}
          onToggleTool={onToggleTool}
          onToggleThinking={onToggleThinking}
          onHitl={onHitl}
        />
      ))}
    </div>
  );
}

function TimelineRow({
  item,
  showThinking,
  onToggleTool,
  onToggleThinking,
  onHitl,
}: {
  item: TimelineItem;
  showThinking: boolean;
  onToggleTool: (toolCallId: string) => void;
  onToggleThinking: (messageId: string) => void;
  onHitl: (
    requestId: string,
    decision: HitlDecision,
    modifiedText?: string,
  ) => void;
}) {
  if (item.kind === "tool") {
    return (
      <div className="rounded-md border border-border bg-muted/40 px-3 py-2 text-sm">
        <div className="flex items-center gap-2">
          <span className="font-mono text-xs">{item.name}</span>
          <StatusPill status={item.status} />
          <button
            type="button"
            className="ml-auto text-xs text-muted-foreground underline"
            onClick={() => onToggleTool(item.id)}
          >
            {item.collapsed ? "展开" : "折叠"}
          </button>
        </div>
        {!item.collapsed && item.args !== undefined ? (
          <pre className="mt-2 overflow-x-auto rounded bg-background p-2 text-xs">
            {item.args}
          </pre>
        ) : null}
        {!item.collapsed && item.result !== undefined ? (
          <pre className="mt-2 overflow-x-auto rounded bg-background p-2 text-xs">
            {item.result}
          </pre>
        ) : null}
      </div>
    );
  }

  if (item.kind === "steer") {
    return (
      <div className="ml-8 rounded-md border border-dashed border-primary/50 bg-primary/5 px-3 py-2 text-sm">
        <div className="mb-1 text-xs font-semibold text-primary">Steering</div>
        <div className="whitespace-pre-wrap">{item.text}</div>
      </div>
    );
  }

  if (item.kind === "hitl") {
    return <HitlCard item={item} onHitl={onHitl} />;
  }

  const isUser = item.role === "user";
  return (
    <div
      className={
        isUser
          ? "ml-8 rounded-lg border border-border bg-card px-3 py-2 text-sm"
          : "mr-8 rounded-lg border border-border bg-muted/30 px-3 py-2 text-sm"
      }
    >
      <div className="mb-1 text-xs text-muted-foreground">
        {isUser ? "用户" : "助手"}
        {item.streaming ? " · 生成中" : ""}
      </div>
      {showThinking && item.thinking ? (
        <details
          className="mb-2 rounded border border-border bg-background/60 px-2 py-1 text-xs text-muted-foreground"
          open={item.thinkingCollapsed === false}
          onToggle={(event) => {
            const open = (event.currentTarget as HTMLDetailsElement).open;
            if (open === (item.thinkingCollapsed ?? true)) {
              onToggleThinking(item.id);
            }
          }}
        >
          <summary className="cursor-pointer select-none font-medium">
            推理过程
            {item.thinkingStreaming ? " …" : ""}
          </summary>
          <div className="mt-1 whitespace-pre-wrap">{item.thinking}</div>
        </details>
      ) : null}
      <MessageBody text={item.text} />
    </div>
  );
}

function HitlCard({
  item,
  onHitl,
}: {
  item: Extract<TimelineItem, { kind: "hitl" }>;
  onHitl: (
    requestId: string,
    decision: HitlDecision,
    modifiedText?: string,
  ) => void;
}) {
  const [modifyText, setModifyText] = useState("");
  const awaiting = item.status === "awaiting";

  return (
    <div className="rounded-md border-2 border-warning-border bg-warning/30 px-3 py-3 text-sm shadow-sm">
      <div className="font-semibold text-warning-foreground">
        需要您确认 · {item.request.title}
      </div>
      <p className="mt-1 whitespace-pre-wrap text-foreground">
        {item.request.context}
      </p>
      <div className="mt-1 text-xs text-muted-foreground">
        状态：
        {item.status === "awaiting"
          ? "等待决策"
          : item.status === "approved"
            ? "已批准"
            : item.status === "rejected"
              ? "已拒绝"
              : item.status === "modified"
                ? "已修改后继续"
                : "已超时"}
      </div>
      {awaiting ? (
        <div className="mt-3 flex flex-col gap-2">
          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              className="rounded-md border border-primary bg-primary px-3 py-1.5 text-xs font-semibold text-primary-foreground"
              onClick={() => onHitl(item.id, "approve")}
            >
              批准
            </button>
            <button
              type="button"
              className="rounded-md border border-destructive bg-destructive px-3 py-1.5 text-xs font-semibold text-destructive-foreground"
              onClick={() => onHitl(item.id, "reject")}
            >
              拒绝
            </button>
          </div>
          <textarea
            className="min-h-[48px] w-full rounded border border-input bg-background px-2 py-1 text-xs"
            placeholder="修改后继续：在此输入改写指令"
            value={modifyText}
            onChange={(event) => setModifyText(event.target.value)}
          />
          <button
            type="button"
            className="self-start rounded-md border border-border bg-card px-3 py-1.5 text-xs font-semibold"
            onClick={() => onHitl(item.id, "modify", modifyText)}
          >
            修改后继续
          </button>
        </div>
      ) : null}
      {item.modifiedText ? (
        <div className="mt-2 text-xs text-muted-foreground">
          改写：{item.modifiedText}
        </div>
      ) : null}
    </div>
  );
}

function MessageBody({ text }: { text: string }) {
  const segments = splitMessageSegments(text);
  return (
    <div className="space-y-2">
      {segments.map((seg) => {
        if (seg.type === "code") {
          const key = `code:${seg.language}:${seg.value.slice(0, 24)}:${seg.value.length}`;
          return (
            <pre
              key={key}
              className="overflow-x-auto rounded bg-background p-2 font-mono text-xs"
            >
              <code>{seg.value}</code>
            </pre>
          );
        }
        const key = `text:${seg.value.slice(0, 24)}:${seg.value.length}`;
        return (
          <div key={key} className="whitespace-pre-wrap">
            {seg.value}
          </div>
        );
      })}
    </div>
  );
}

function StatusPill({ status }: { status: "running" | "success" | "error" }) {
  const label =
    status === "running" ? "调用中" : status === "success" ? "成功" : "失败";
  return (
    <span className="rounded-full border border-border px-2 py-0.5 text-xs">
      {label}
    </span>
  );
}
