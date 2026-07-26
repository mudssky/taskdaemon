import type { RuntimeCapabilities } from "@taskdaemon/agent-protocol";
import { shouldShowThinking } from "./capability-ui";
import { splitMessageSegments } from "./message-format";
import type { StreamViewState, TimelineItem } from "./stream-reducer";

type TimelineProps = {
  state: StreamViewState;
  capabilities: RuntimeCapabilities | undefined;
  onToggleTool: (toolCallId: string) => void;
};

/**
 * 消息 + 工具时间线。
 *
 * 参数:
 *   - props.state / capabilities / onToggleTool。
 *
 * 返回值:
 *   - 时间线节点。
 */
export function Timeline({ state, capabilities, onToggleTool }: TimelineProps) {
  const showThinking = shouldShowThinking(capabilities);

  if (state.items.length === 0) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
        还没有消息。发送第一条开始对话。
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-3 p-4">
      {state.items.map((item) => (
        <TimelineRow
          key={`${item.kind}-${item.id}`}
          item={item}
          showThinking={showThinking}
          onToggleTool={onToggleTool}
        />
      ))}
    </div>
  );
}

function TimelineRow({
  item,
  showThinking,
  onToggleTool,
}: {
  item: TimelineItem;
  showThinking: boolean;
  onToggleTool: (toolCallId: string) => void;
}) {
  if (item.kind === "tool") {
    return (
      <div className="rounded-md border border-border bg-muted/40 px-3 py-2 text-sm">
        <div className="flex items-center gap-2">
          <span className="font-mono text-xs uppercase text-muted-foreground">
            tool
          </span>
          <span className="font-semibold">{item.name}</span>
          <StatusPill status={item.status} />
          {item.result !== undefined ? (
            <button
              type="button"
              className="ml-auto text-xs text-primary underline"
              onClick={() => onToggleTool(item.id)}
            >
              {item.collapsed ? "展开结果" : "折叠结果"}
            </button>
          ) : null}
        </div>
        {item.args !== undefined ? (
          <pre className="mt-2 overflow-x-auto rounded bg-background p-2 text-xs">
            {item.args}
          </pre>
        ) : null}
        {item.result !== undefined && !item.collapsed ? (
          <pre className="mt-2 overflow-x-auto rounded bg-background p-2 text-xs">
            {item.result}
          </pre>
        ) : null}
        {item.result !== undefined && item.collapsed ? (
          <p className="mt-1 text-xs text-muted-foreground">
            结果已折叠（{item.result.length} 字符）
          </p>
        ) : null}
      </div>
    );
  }

  const isUser = item.role === "user";
  return (
    <div
      className={
        isUser
          ? "ml-8 rounded-md border border-primary/30 bg-primary/5 px-3 py-2"
          : "mr-8 rounded-md border border-border bg-card px-3 py-2"
      }
    >
      <div className="mb-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
        {item.role}
        {item.streaming ? " · 流式中" : ""}
      </div>
      {showThinking && item.thinking ? (
        <details className="mb-2 rounded border border-warning-border bg-warning/40 px-2 py-1 text-xs">
          <summary className="cursor-pointer font-medium">
            推理
            {item.thinkingStreaming ? " …" : ""}
          </summary>
          <pre className="mt-1 whitespace-pre-wrap">{item.thinking}</pre>
        </details>
      ) : null}
      <MessageBody text={item.text} />
    </div>
  );
}

function MessageBody({ text }: { text: string }) {
  const segments = splitMessageSegments(text);
  return (
    <div className="space-y-2 text-sm leading-relaxed">
      {segments.map((segment) => {
        const key =
          segment.type === "code"
            ? `code:${segment.language}:${segment.value}`
            : `text:${segment.value}`;
        if (segment.type === "code") {
          return (
            <pre
              key={key}
              className="overflow-x-auto rounded-md bg-zinc-900 p-3 text-xs text-zinc-100"
            >
              {segment.language ? (
                <div className="mb-1 text-[10px] uppercase text-zinc-400">
                  {segment.language}
                </div>
              ) : null}
              <code>{segment.value}</code>
            </pre>
          );
        }
        return (
          <p key={key} className="whitespace-pre-wrap">
            {segment.value}
          </p>
        );
      })}
    </div>
  );
}

function StatusPill({ status }: { status: "running" | "success" | "error" }) {
  const label =
    status === "running" ? "调用中" : status === "success" ? "成功" : "失败";
  const className =
    status === "running"
      ? "bg-warning text-warning-foreground"
      : status === "success"
        ? "bg-success text-success-foreground"
        : "bg-destructive-soft text-destructive-soft-foreground";
  return (
    <span
      className={`rounded-full px-2 py-0.5 text-[10px] font-medium ${className}`}
    >
      {label}
    </span>
  );
}
