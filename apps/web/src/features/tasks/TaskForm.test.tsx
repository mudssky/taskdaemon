import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { TaskForm } from "./TaskForm";

describe("TaskForm", () => {
  it("submits a structured inline runner payload", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn(async () => undefined);
    render(<TaskForm task={null} isSubmitting={false} onSubmit={onSubmit} />);

    await user.type(screen.getByLabelText("任务名称"), "backup");
    await user.type(screen.getByLabelText("命令片段"), "echo ok");
    await user.click(screen.getByRole("button", { name: "创建任务" }));

    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({
        name: "backup",
        cronExpression: "30 9 * * *",
        runner: expect.objectContaining({
          type: "shell",
          inline: "echo ok",
          timeoutSeconds: 3600,
        }),
      }),
    );
  });

  it("blocks high frequency cron until confirmed", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn(async () => undefined);
    render(<TaskForm task={null} isSubmitting={false} onSubmit={onSubmit} />);

    const cronInput = screen.getByLabelText("cron");
    await user.clear(cronInput);
    await user.type(cronInput, "*/5 * * * * *");
    await user.type(screen.getByLabelText("任务名称"), "fast");
    await user.type(screen.getByLabelText("命令片段"), "echo ok");
    await user.click(screen.getByRole("button", { name: "创建任务" }));

    expect(onSubmit).not.toHaveBeenCalled();
    await user.click(screen.getByLabelText("我确认这个调度频率符合预期"));
    await user.click(screen.getByRole("button", { name: "创建任务" }));

    expect(onSubmit).toHaveBeenCalledOnce();
  });
});
