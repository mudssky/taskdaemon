/**
 * 消息文本基础渲染辅助：换行与 fenced code 块。
 * 不做完整 Markdown 解析（G4 最小）。
 */

export type TextSegment =
  | { type: "text"; value: string }
  | { type: "code"; language: string; value: string };

/**
 * 将纯文本切成文本段与 ``` 代码块。
 *
 * 参数:
 *   - source: 原始消息文本。
 *
 * 返回值:
 *   - 段列表。
 */
export function splitMessageSegments(source: string): TextSegment[] {
  const segments: TextSegment[] = [];
  const fence = /```([^\n`]*)\n?([\s\S]*?)```/g;
  let lastIndex = 0;
  let match = fence.exec(source);

  while (match !== null) {
    if (match.index > lastIndex) {
      segments.push({
        type: "text",
        value: source.slice(lastIndex, match.index),
      });
    }
    segments.push({
      type: "code",
      language: (match[1] ?? "").trim(),
      value: match[2] ?? "",
    });
    lastIndex = match.index + match[0].length;
    match = fence.exec(source);
  }

  if (lastIndex < source.length) {
    segments.push({ type: "text", value: source.slice(lastIndex) });
  }

  if (segments.length === 0) {
    return [{ type: "text", value: source }];
  }
  return segments;
}

/**
 * 相对时间展示（列表用）。
 *
 * 参数:
 *   - iso: ISO 时间字符串。
 *   - now: 可选当前时间。
 *
 * 返回值:
 *   - 简短中文相对/绝对描述。
 */
export function formatActivityTime(
  iso: string,
  now: Date = new Date(),
): string {
  const then = new Date(iso);
  if (Number.isNaN(then.getTime())) {
    return iso;
  }
  const diffMs = now.getTime() - then.getTime();
  const minute = 60_000;
  const hour = 60 * minute;
  const day = 24 * hour;

  if (diffMs < minute) {
    return "刚刚";
  }
  if (diffMs < hour) {
    return `${Math.floor(diffMs / minute)} 分钟前`;
  }
  if (diffMs < day) {
    return `${Math.floor(diffMs / hour)} 小时前`;
  }
  return then.toLocaleString();
}

/**
 * Thread 状态文案。
 *
 * 参数:
 *   - status: ThreadStatus。
 *
 * 返回值:
 *   - 中文标签。
 */
export function threadStatusLabel(status: string): string {
  switch (status) {
    case "idle":
      return "空闲";
    case "busy":
      return "运行中";
    case "interrupted":
      return "已中断";
    case "error":
      return "出错";
    case "archived":
      return "已归档";
    case "deleted":
      return "已删除";
    default:
      return status;
  }
}
