import type { Thread } from "@taskdaemon/agent-protocol";
import { useState } from "react";
import type { FileChangeItem } from "./stream-reducer";

type WorkspacePanelProps = {
  thread: Thread | undefined;
  fileChanges: FileChangeItem[];
  /** capabilities.workspaceBinding && thread.workspaceBound */
  visible: boolean;
};

/**
 * 工作区只读面板：cwd + 文件变更 + diff。
 *
 * 参数:
 *   - props: thread / 变更列表 / 可见性。
 *
 * 返回值:
 *   - 面板节点；不可见时 null。
 */
export function WorkspacePanel({
  thread,
  fileChanges,
  visible,
}: WorkspacePanelProps) {
  const [selectedId, setSelectedId] = useState<string | null>(null);

  if (!visible) {
    return null;
  }

  const selected =
    fileChanges.find((f) => f.id === selectedId) ?? fileChanges.at(-1) ?? null;

  return (
    <div className="border-b border-border bg-card/80 px-4 py-2 text-xs">
      <div className="flex flex-wrap items-center gap-2">
        <span className="font-semibold text-foreground">工作区</span>
        <code className="rounded bg-muted px-2 py-0.5">
          {thread?.workspaceRoot ?? "(未绑定路径)"}
        </code>
        <span className="text-muted-foreground">只读预览 · 不可编辑</span>
      </div>
      {fileChanges.length === 0 ? (
        <p className="mt-1 text-muted-foreground">
          尚无文件变更。输入含 file / edit 的消息可演示。
        </p>
      ) : (
        <div className="mt-2 grid gap-2 md:grid-cols-[12rem_1fr]">
          <ul className="max-h-36 space-y-1 overflow-y-auto">
            {fileChanges.map((change) => (
              <li key={change.id}>
                <button
                  type="button"
                  className={
                    selected?.id === change.id
                      ? "w-full rounded border border-primary bg-primary/10 px-2 py-1 text-left"
                      : "w-full rounded border border-transparent px-2 py-1 text-left hover:bg-muted"
                  }
                  onClick={() => setSelectedId(change.id)}
                >
                  <span className="font-mono">{change.path}</span>
                  <span className="ml-1 text-muted-foreground">
                    {change.kind}
                  </span>
                  {change.outsideSandbox ? (
                    <span className="ml-1 text-destructive">越界</span>
                  ) : null}
                </button>
              </li>
            ))}
          </ul>
          <div className="max-h-36 overflow-auto rounded border border-border bg-background p-2">
            {selected?.outsideSandbox ? (
              <p className="text-destructive">
                路径超出会话沙箱根，已拒绝写入预览；仅展示告警。
              </p>
            ) : null}
            {selected?.diff ? (
              <pre className="whitespace-pre-wrap font-mono text-[11px] leading-relaxed">
                {selected.diff}
              </pre>
            ) : (
              <p className="text-muted-foreground">无 diff 内容</p>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
