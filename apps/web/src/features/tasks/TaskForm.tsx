import { zodResolver } from "@hookform/resolvers/zod";
import { Save } from "lucide-react";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
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
        <Field className="span-2" id="task-name" label="任务名称">
          <Input
            id="task-name"
            {...form.register("name")}
            placeholder="PostgreSQL backup"
          />
          <ErrorMessage message={form.formState.errors.name?.message} />
        </Field>
        <Field id="task-cron" label="cron">
          <Input
            id="task-cron"
            className="mono"
            {...form.register("cronExpression")}
            placeholder="30 9 * * *"
          />
          <ErrorMessage
            message={form.formState.errors.cronExpression?.message}
          />
        </Field>
        <Field id="task-timezone" label="timezone">
          <Input
            id="task-timezone"
            {...form.register("timezone")}
            placeholder="Asia/Hong_Kong"
          />
        </Field>
        <Field className="span-2" id="task-description" label="描述">
          <Textarea
            id="task-description"
            rows={2}
            {...form.register("description")}
          />
        </Field>
      </div>

      {cron.ok && cron.warnings.length > 0 ? (
        <div className="warning-box">
          <strong>cron 风险确认</strong>
          {cron.warnings.map((warning) => (
            <p key={warning}>{warningLabel(warning)}</p>
          ))}
          <CheckboxRow
            checked={values.confirmCronWarnings}
            label="我确认这个调度频率符合预期"
            onCheckedChange={(checked) =>
              form.setValue("confirmCronWarnings", checked, {
                shouldDirty: true,
                shouldValidate: true,
              })
            }
          />
          <ErrorMessage
            message={form.formState.errors.confirmCronWarnings?.message}
          />
        </div>
      ) : null}

      <fieldset className="fieldset">
        <legend>Runner</legend>
        <div className="form-grid">
          <Field id="task-runner-type" label="类型">
            <Select
              value={values.runnerType}
              onValueChange={(value) =>
                form.setValue(
                  "runnerType",
                  value as TaskFormValues["runnerType"],
                  {
                    shouldDirty: true,
                    shouldValidate: true,
                  },
                )
              }
            >
              <SelectTrigger id="task-runner-type">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  {runnerTypes.map((type) => (
                    <SelectItem key={type} value={type}>
                      {runnerTypeLabel(type)}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </Field>
          <Field id="task-command-mode" label="命令来源">
            <Select
              value={values.commandMode}
              onValueChange={(value) =>
                form.setValue(
                  "commandMode",
                  value as TaskFormValues["commandMode"],
                  {
                    shouldDirty: true,
                    shouldValidate: true,
                  },
                )
              }
            >
              <SelectTrigger id="task-command-mode">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  <SelectItem value="inline">命令片段</SelectItem>
                  <SelectItem value="script">脚本路径</SelectItem>
                </SelectGroup>
              </SelectContent>
            </Select>
          </Field>
          {values.commandMode === "inline" ? (
            <Field className="span-2" id="task-inline" label="命令片段">
              <Textarea
                id="task-inline"
                className="mono"
                rows={4}
                {...form.register("inline")}
              />
              <ErrorMessage message={form.formState.errors.inline?.message} />
            </Field>
          ) : (
            <Field className="span-2" id="task-script-path" label="脚本路径">
              <Input
                id="task-script-path"
                className="mono"
                {...form.register("scriptPath")}
              />
              <ErrorMessage
                message={form.formState.errors.scriptPath?.message}
              />
            </Field>
          )}
          <Field id="task-args" label="参数">
            <Input
              id="task-args"
              className="mono"
              {...form.register("argsText")}
              placeholder="--full --quiet"
            />
          </Field>
          <Field id="task-work-dir" label="工作目录">
            <Input
              id="task-work-dir"
              className="mono"
              {...form.register("workDir")}
            />
          </Field>
          <Field id="task-timeout" label="timeout 秒">
            <Input
              id="task-timeout"
              type="number"
              min={1}
              {...form.register("timeoutSeconds", { valueAsNumber: true })}
            />
            <ErrorMessage
              message={form.formState.errors.timeoutSeconds?.message}
            />
          </Field>
          <Field id="task-output-limit" label="输出截断字节">
            <Input
              id="task-output-limit"
              type="number"
              min={0}
              {...form.register("outputLimitBytes", { valueAsNumber: true })}
            />
          </Field>
          <Field className="span-2" id="task-env" label="环境变量">
            <Textarea
              id="task-env"
              className="mono"
              rows={3}
              {...form.register("envText")}
              placeholder="KEY=value"
            />
          </Field>
        </div>
      </fieldset>

      <div className="form-actions">
        <CheckboxRow
          checked={values.enabled}
          label="保存后启用任务"
          onCheckedChange={(checked) =>
            form.setValue("enabled", checked, {
              shouldDirty: true,
              shouldValidate: true,
            })
          }
        />
        <Button variant="primary" type="submit" disabled={isSubmitting}>
          <Save aria-hidden="true" data-icon="inline-start" />
          {isSubmitting ? "保存中" : task ? "保存修改" : "创建任务"}
        </Button>
      </div>
    </form>
  );
}

function Field({
  id,
  label,
  className,
  children,
}: {
  id: string;
  label: string;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <div className={`field ${className ?? ""}`.trim()}>
      <Label htmlFor={id}>{label}</Label>
      {children}
    </div>
  );
}

function CheckboxRow({
  checked,
  label,
  onCheckedChange,
}: {
  checked: boolean;
  label: string;
  onCheckedChange: (checked: boolean) => void;
}) {
  return (
    <Label className="check-row">
      <Checkbox
        checked={checked}
        onCheckedChange={(value) => onCheckedChange(value === true)}
      />
      {label}
    </Label>
  );
}

function ErrorMessage({ message }: { message?: string }) {
  return message ? <span className="field-error">{message}</span> : null;
}
