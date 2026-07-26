type ComposerProps = {
  disabled: boolean;
  running: boolean;
  onSend: (text: string) => void;
  onCancel: () => void;
};

/**
 * 输入区：发送与中断。
 *
 * 参数:
 *   - props.disabled / running / onSend / onCancel。
 *
 * 返回值:
 *   - 输入区节点。
 */
export function Composer({
  disabled,
  running,
  onSend,
  onCancel,
}: ComposerProps) {
  return (
    <form
      className="flex gap-2 border-t border-border bg-card p-3"
      onSubmit={(event) => {
        event.preventDefault();
        if (disabled || running) {
          return;
        }
        const form = event.currentTarget;
        const data = new FormData(form);
        const text = String(data.get("message") ?? "");
        if (!text.trim()) {
          return;
        }
        onSend(text);
        form.reset();
      }}
    >
      <textarea
        name="message"
        rows={2}
        disabled={disabled || running}
        placeholder={
          running
            ? "生成中… 可中断"
            : disabled
              ? "请选择或创建会话"
              : "输入消息。含 tool 可演示工具调用；含 code 可演示代码块。"
        }
        className="min-h-[56px] flex-1 resize-y rounded-md border border-input bg-background px-3 py-2 text-sm outline-none focus:border-ring"
        onKeyDown={(event) => {
          if (event.key === "Enter" && !event.shiftKey) {
            event.preventDefault();
            event.currentTarget.form?.requestSubmit();
          }
        }}
      />
      <div className="flex flex-col gap-2">
        {running ? (
          <button
            type="button"
            className="rounded-md border border-destructive bg-destructive px-3 py-2 text-sm font-semibold text-destructive-foreground"
            onClick={onCancel}
          >
            中断
          </button>
        ) : (
          <button
            type="submit"
            disabled={disabled}
            className="rounded-md border border-primary bg-primary px-3 py-2 text-sm font-semibold text-primary-foreground disabled:opacity-50"
          >
            发送
          </button>
        )}
      </div>
    </form>
  );
}
