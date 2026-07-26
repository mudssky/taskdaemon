type ComposerProps = {
  disabled: boolean;
  running: boolean;
  awaitingHuman: boolean;
  steeringEnabled: boolean;
  onSend: (text: string) => void;
  onSteer: (text: string) => void;
  onCancel: () => void;
};

/**
 * 输入区：发送 / steering / 中断。
 *
 * 参数:
 *   - props: 状态与回调。
 *
 * 返回值:
 *   - 输入区节点。
 */
export function Composer({
  disabled,
  running,
  awaitingHuman,
  steeringEnabled,
  onSend,
  onSteer,
  onCancel,
}: ComposerProps) {
  const canSteer = running && steeringEnabled && !awaitingHuman;
  const canSend = !disabled && !running && !awaitingHuman;

  return (
    <form
      className="flex gap-2 border-t border-border bg-card p-3"
      onSubmit={(event) => {
        event.preventDefault();
        const form = event.currentTarget;
        const data = new FormData(form);
        const text = String(data.get("message") ?? "");
        if (!text.trim()) {
          return;
        }
        if (canSteer) {
          onSteer(text);
          form.reset();
          return;
        }
        if (!canSend) {
          return;
        }
        onSend(text);
        form.reset();
      }}
    >
      <textarea
        name="message"
        rows={2}
        disabled={disabled || awaitingHuman}
        placeholder={
          awaitingHuman
            ? "等待您在时间线中确认 HITL…"
            : canSteer
              ? "运行中 · 输入 steering 注入（不中断 run）"
              : running
                ? "生成中… 可中断"
                : disabled
                  ? "请选择或创建会话"
                  : "输入消息。hitl / file / tool / code 可触发 mock 演示。"
        }
        className="min-h-[56px] flex-1 resize-y rounded-md border border-input bg-background px-3 py-2 text-sm outline-none focus:border-ring disabled:opacity-60"
        onKeyDown={(event) => {
          if (event.key === "Enter" && !event.shiftKey) {
            event.preventDefault();
            event.currentTarget.form?.requestSubmit();
          }
        }}
      />
      <div className="flex flex-col gap-2">
        {running || awaitingHuman ? (
          <>
            {canSteer ? (
              <button
                type="submit"
                className="rounded-md border border-primary bg-primary px-3 py-2 text-sm font-semibold text-primary-foreground"
              >
                注入
              </button>
            ) : null}
            <button
              type="button"
              className="rounded-md border border-destructive bg-destructive px-3 py-2 text-sm font-semibold text-destructive-foreground"
              onClick={onCancel}
            >
              中断
            </button>
          </>
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
