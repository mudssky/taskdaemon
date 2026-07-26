/**
 * JSONL 分帧器：半包、粘包、超长行、非法 JSON。
 * G0：禁用 Node readline；严格按 \n 分帧。
 */

export type JsonlFrameHandler = {
  /**
   * 合法 JSON 对象帧。
   *
   * 参数:
   *   - value: 解析后的对象。
   *   - raw: 原始行文本。
   *
   * 返回值:
   *   - 无。
   */
  onFrame(value: unknown, raw: string): void;
  /**
   * 非法 JSON 行（进程通常不退出，调用方记录后继续）。
   *
   * 参数:
   *   - raw: 原始行。
   *   - error: 解析错误。
   *
   * 返回值:
   *   - 无。
   */
  onInvalid?(raw: string, error: Error): void;
  /**
   * 超长行被丢弃或截断时回调。
   *
   * 参数:
   *   - length: 已缓冲字节/字符长度。
   *   - action: drop | flush-invalid。
   *
   * 返回值:
   *   - 无。
   */
  onOversize?(length: number, action: "drop" | "flush-invalid"): void;
};

export type JsonlFramerOptions = {
  /** 单行最大字符数，默认 8MiB（覆盖 G0 ~2MB prompt）。 */
  maxLineChars?: number;
};

/**
 * 创建 JSONL 分帧器。
 *
 * 参数:
 *   - handler: 帧回调。
 *   - options: 超长行阈值等。
 *
 * 返回值:
 *   - { push, flush, reset, pendingChars }。
 */
export function createJsonlFramer(
  handler: JsonlFrameHandler,
  options: JsonlFramerOptions = {},
) {
  const maxLineChars = options.maxLineChars ?? 8 * 1024 * 1024;
  let buffer = "";

  /**
   * 处理完整一行。
   *
   * 参数:
   *   - line: 不含 \n 的行。
   *
   * 返回值:
   *   - 无。
   */
  function handleLine(line: string): void {
    if (line.endsWith("\r")) line = line.slice(0, -1);
    if (line.length === 0) return;
    if (line.length > maxLineChars) {
      handler.onOversize?.(line.length, "flush-invalid");
      handler.onInvalid?.(
        line.slice(0, 256),
        new Error(`line exceeds maxLineChars=${maxLineChars}`),
      );
      return;
    }
    try {
      const value = JSON.parse(line) as unknown;
      handler.onFrame(value, line);
    } catch (err) {
      const error = err instanceof Error ? err : new Error(String(err));
      handler.onInvalid?.(line, error);
    }
  }

  return {
    /**
     * 追加 stdout 字节/字符串。
     *
     * 参数:
     *   - chunk: 半包或粘包数据。
     *
     * 返回值:
     *   - 本次解析出的完整帧数。
     */
    push(chunk: string | Buffer): number {
      buffer += typeof chunk === "string" ? chunk : chunk.toString("utf8");
      // 超长未完成行：丢弃缓冲，防止 OOM
      if (buffer.length > maxLineChars && !buffer.includes("\n")) {
        const len = buffer.length;
        buffer = "";
        handler.onOversize?.(len, "drop");
        handler.onInvalid?.(
          "",
          new Error(`unterminated line exceeded maxLineChars=${maxLineChars}`),
        );
        return 0;
      }
      let frames = 0;
      while (true) {
        const idx = buffer.indexOf("\n");
        if (idx < 0) break;
        const line = buffer.slice(0, idx);
        buffer = buffer.slice(idx + 1);
        handleLine(line);
        frames += 1;
      }
      return frames;
    },

    /**
     * 流结束时刷掉无换行尾帧（若有）。
     *
     * 参数: 无。
     * 返回值: 无。
     */
    flush(): void {
      if (buffer.length === 0) return;
      const rest = buffer;
      buffer = "";
      handleLine(rest);
    },

    /**
     * 重置缓冲。
     *
     * 参数: 无。
     * 返回值: 无。
     */
    reset(): void {
      buffer = "";
    },

    /**
     * 当前未完成行的字符数。
     *
     * 参数: 无。
     * 返回值: 缓冲长度。
     */
    pendingChars(): number {
      return buffer.length;
    },
  };
}

export type JsonlFramer = {
  push(chunk: string | Buffer): number;
  flush(): void;
  reset(): void;
  pendingChars(): number;
};
