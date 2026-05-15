import { zodResolver } from "@hookform/resolvers/zod";
import { Save } from "lucide-react";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { runnerTypes, type Task } from "../../lib/api/types";
import { validateCronExpression, warningLabel } from "../scheduler/cron";
import {
  defaultTaskFormValues,
  type TaskFormValues,
  taskFormSchema,
  toTaskPayload,
} from "./task.schema";
import { runnerTypeLabel } from "./task-format";

type TaskFormProps = {
  task: Task | null;
  isSubmitting: boolean;
  onSubmit: (values: ReturnType<typeof toTaskPayload>) => Promise<void>;
};

export function TaskForm({ task, isSubmitting, onSubmit }: TaskFormProps) {
  const form = useForm<TaskFormValues>({
    resolver: zodResolver(taskFormSchema),
    defaultValues: defaultTaskFormValues(task),
    mode: "onChange",
  });
  const values = form.watch();
  const cron = validateCronExpression(values.cronExpression, values.timezone);

  useEffect(() => {
    form.reset(defaultTaskFormValues(task));
  }, [form, task]);

  return (
    <form
      className="task-form"
      onSubmit={form.handleSubmit(async (formValues) => {
        await onSubmit(toTaskPayload(formValues));
      })}
    >
      <div className="form-grid">
        <label className="field span-2">
          任务名称
          <input {...form.register("name")} placeholder="PostgreSQL backup" />
          <ErrorMessage message={form.formState.errors.name?.message} />
        </label>
        <label className="field">
          cron
          <input
            className="mono"
            {...form.register("cronExpression")}
            placeholder="30 9 * * *"
          />
          <ErrorMessage
            message={form.formState.errors.cronExpression?.message}
          />
        </label>
        <label className="field">
          timezone
          <input {...form.register("timezone")} placeholder="Asia/Hong_Kong" />
        </label>
        <label className="field span-2">
          描述
          <textarea rows={2} {...form.register("description")} />
        </label>
      </div>

      {cron.ok && cron.warnings.length > 0 ? (
        <div className="warning-box">
          <strong>cron 风险确认</strong>
          {cron.warnings.map((warning) => (
            <p key={warning}>{warningLabel(warning)}</p>
          ))}
          <label className="check-row">
            <input type="checkbox" {...form.register("confirmCronWarnings")} />
            我确认这个调度频率符合预期
          </label>
          <ErrorMessage
            message={form.formState.errors.confirmCronWarnings?.message}
          />
        </div>
      ) : null}

      <fieldset className="fieldset">
        <legend>Runner</legend>
        <div className="form-grid">
          <label className="field">
            类型
            <select {...form.register("runnerType")}>
              {runnerTypes.map((type) => (
                <option key={type} value={type}>
                  {runnerTypeLabel(type)}
                </option>
              ))}
            </select>
          </label>
          <label className="field">
            命令来源
            <select {...form.register("commandMode")}>
              <option value="inline">命令片段</option>
              <option value="script">脚本路径</option>
            </select>
          </label>
          {values.commandMode === "inline" ? (
            <label className="field span-2">
              命令片段
              <textarea
                className="mono"
                rows={4}
                {...form.register("inline")}
              />
              <ErrorMessage message={form.formState.errors.inline?.message} />
            </label>
          ) : (
            <label className="field span-2">
              脚本路径
              <input className="mono" {...form.register("scriptPath")} />
              <ErrorMessage
                message={form.formState.errors.scriptPath?.message}
              />
            </label>
          )}
          <label className="field">
            参数
            <input
              className="mono"
              {...form.register("argsText")}
              placeholder="--full --quiet"
            />
          </label>
          <label className="field">
            工作目录
            <input className="mono" {...form.register("workDir")} />
          </label>
          <label className="field">
            timeout 秒
            <input
              type="number"
              min={1}
              {...form.register("timeoutSeconds", { valueAsNumber: true })}
            />
            <ErrorMessage
              message={form.formState.errors.timeoutSeconds?.message}
            />
          </label>
          <label className="field">
            输出截断字节
            <input
              type="number"
              min={0}
              {...form.register("outputLimitBytes", { valueAsNumber: true })}
            />
          </label>
          <label className="field span-2">
            环境变量
            <textarea
              className="mono"
              rows={3}
              {...form.register("envText")}
              placeholder="KEY=value"
            />
          </label>
        </div>
      </fieldset>

      <div className="form-actions">
        <label className="check-row">
          <input type="checkbox" {...form.register("enabled")} />
          保存后启用任务
        </label>
        <button
          className="button primary"
          type="submit"
          disabled={isSubmitting}
        >
          <Save aria-hidden="true" size={16} />
          {isSubmitting ? "保存中" : task ? "保存修改" : "创建任务"}
        </button>
      </div>
    </form>
  );
}

function ErrorMessage({ message }: { message?: string }) {
  return message ? <span className="field-error">{message}</span> : null;
}
