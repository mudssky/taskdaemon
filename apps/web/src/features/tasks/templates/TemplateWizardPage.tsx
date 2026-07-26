import { zodResolver } from "@hookform/resolvers/zod";
import { useNavigate } from "@tanstack/react-router";
import {
  ArrowLeft,
  ArrowRight,
  Check,
  ClipboardCopy,
  LayoutTemplate,
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { useForm } from "react-hook-form";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { ApiClientError } from "../../../lib/api/client";
import type {
  TemplateDefinition,
  TemplateTaskDraft,
} from "../../../lib/api/types";
import { useCreateTaskMutation } from "../tasks.queries";
import { TemplateParamFields } from "./TemplateParamFields";
import {
  areParamValuesDirty,
  buildParamSchema,
  defaultParamValues,
  draftToTaskPayload,
  extractTemplateFieldErrors,
  mapTemplateFieldErrors,
  type TemplateParamValues,
  toRenderParams,
} from "./template.schema";
import {
  isWizardDirty,
  prevWizardStep,
  WIZARD_STEP_COUNT,
  type WizardStep,
  wizardStepMeta,
  wizardStepNumber,
} from "./template-wizard";
import {
  useRenderTemplateMutation,
  useTemplatesQuery,
} from "./templates.queries";

/**
 * 备份模板多步向导页：选模板 → 填参 → 预览 → 走既有 createTask。
 *
 * @returns 向导页面。
 */
export function TemplateWizardPage() {
  const navigate = useNavigate();
  const templatesQuery = useTemplatesQuery();
  const createTask = useCreateTaskMutation();
  const renderTemplate = useRenderTemplateMutation();

  const [step, setStep] = useState<WizardStep>("select");
  const [templateId, setTemplateId] = useState<string | null>(null);
  const [draft, setDraft] = useState<TemplateTaskDraft | null>(null);
  const [pageError, setPageError] = useState<string | null>(null);
  const [traceId, setTraceId] = useState<string | null>(null);
  const [leaveOpen, setLeaveOpen] = useState(false);
  const [copyDone, setCopyDone] = useState(false);

  const selectedTemplate = useMemo(() => {
    const list = templatesQuery.data ?? [];
    return list.find((item) => item.id === templateId) ?? null;
  }, [templatesQuery.data, templateId]);

  const paramSchema = useMemo(
    () => buildParamSchema(selectedTemplate?.params ?? []),
    [selectedTemplate],
  );

  const form = useForm<TemplateParamValues>({
    resolver: zodResolver(paramSchema),
    defaultValues: {},
    mode: "onBlur",
  });

  const watchedValues = form.watch();
  const paramsDirty = selectedTemplate
    ? areParamValuesDirty(selectedTemplate.params, watchedValues)
    : false;
  const dirty = isWizardDirty(templateId, paramsDirty, step);

  useEffect(() => {
    if (!selectedTemplate) {
      return;
    }
    form.reset(defaultParamValues(selectedTemplate.params));
  }, [selectedTemplate, form]);

  useEffect(() => {
    if (!dirty) {
      return;
    }
    function onBeforeUnload(event: BeforeUnloadEvent) {
      event.preventDefault();
      event.returnValue = "";
    }
    window.addEventListener("beforeunload", onBeforeUnload);
    return () => window.removeEventListener("beforeunload", onBeforeUnload);
  }, [dirty]);

  /**
   * 选择模板并进入参数步。
   *
   * @param template - 选中的模板。
   */
  function selectTemplate(template: TemplateDefinition) {
    setTemplateId(template.id);
    setDraft(null);
    setPageError(null);
    setTraceId(null);
    setStep("params");
  }

  /**
   * 客户端校验后请求 render，进入预览步。
   */
  async function goPreview() {
    if (!selectedTemplate || !templateId) {
      return;
    }
    setPageError(null);
    setTraceId(null);
    const valid = await form.trigger();
    if (!valid) {
      return;
    }
    const values = form.getValues();
    try {
      const result = await renderTemplate.mutateAsync({
        id: templateId,
        body: {
          params: toRenderParams(selectedTemplate.params, values),
        },
      });
      setDraft(result);
      setStep("preview");
    } catch (error) {
      applyMutationError(error, true);
    }
  }

  /**
   * 将草稿提交到既有任务创建接口。
   */
  async function submitDraft() {
    if (!draft) {
      return;
    }
    setPageError(null);
    setTraceId(null);
    try {
      await createTask.mutateAsync(draftToTaskPayload(draft));
      await navigate({ to: "/tasks" });
    } catch (error) {
      applyMutationError(error, false);
    }
  }

  /**
   * 统一映射 mutation 错误到字段或页面级提示。
   *
   * @param error - 捕获的错误。
   * @param preferFieldMapping - 是否优先映射字段错误并回到 params。
   */
  function applyMutationError(error: unknown, preferFieldMapping: boolean) {
    if (error instanceof ApiClientError) {
      setTraceId(error.traceId);
      const fieldMap = mapTemplateFieldErrors(
        extractTemplateFieldErrors(error.details),
      );
      const fieldNames = Object.keys(fieldMap);
      if (preferFieldMapping && fieldNames.length > 0) {
        for (const name of fieldNames) {
          form.setError(name, { type: "server", message: fieldMap[name] });
        }
        setStep("params");
        setPageError("请修正标出的参数后重试。");
        return;
      }
      setPageError(errorMessageForCode(error.code));
      return;
    }
    setPageError("操作失败，请稍后重试。");
  }

  /**
   * 复制命令预览（不改动原文，不解掩码）。
   */
  async function copyPreview() {
    if (!draft?.commandPreview) {
      return;
    }
    try {
      await navigator.clipboard.writeText(draft.commandPreview);
      setCopyDone(true);
      window.setTimeout(() => setCopyDone(false), 1500);
    } catch {
      setPageError("复制失败，请手动选择预览文本。");
    }
  }

  const stepNumber = wizardStepNumber(step);
  const busy =
    renderTemplate.isPending ||
    createTask.isPending ||
    form.formState.isSubmitting;

  return (
    <AlertDialog open={leaveOpen} onOpenChange={setLeaveOpen}>
      <section className="panel form-panel wide" aria-labelledby="wizard-title">
        <div className="panel-header">
          <div>
            <h2 id="wizard-title">从模板创建任务</h2>
            <p>
              选择备份模板、填写参数并预览命令后，再提交到既有任务创建流程。
            </p>
          </div>
          <Button
            variant="subtle"
            type="button"
            onClick={() => {
              if (dirty) {
                setLeaveOpen(true);
                return;
              }
              void navigate({ to: "/tasks" });
            }}
          >
            <ArrowLeft aria-hidden="true" data-icon="inline-start" />
            返回列表
          </Button>
        </div>

        <ol className="wizard-steps" aria-label="向导进度">
          {(
            Object.entries(wizardStepMeta) as [
              WizardStep,
              (typeof wizardStepMeta)[WizardStep],
            ][]
          ).map(([key, meta]) => (
            <li
              key={key}
              className={
                key === step
                  ? "wizard-step current"
                  : meta.index < stepNumber
                    ? "wizard-step done"
                    : "wizard-step"
              }
              aria-current={key === step ? "step" : undefined}
            >
              <span className="wizard-step-index">
                {meta.index}/{WIZARD_STEP_COUNT}
              </span>
              <span>{meta.label}</span>
            </li>
          ))}
        </ol>

        {pageError ? (
          <div className="empty-state error" role="alert">
            <p>{pageError}</p>
            {traceId ? (
              <p className="trace-id">
                traceId: <code>{traceId}</code>
              </p>
            ) : null}
          </div>
        ) : null}

        {step === "select" ? (
          <SelectStep
            loading={templatesQuery.isLoading}
            error={templatesQuery.isError}
            traceId={
              templatesQuery.error instanceof ApiClientError
                ? templatesQuery.error.traceId
                : null
            }
            templates={templatesQuery.data ?? []}
            selectedId={templateId}
            onSelect={selectTemplate}
            onRetry={() => void templatesQuery.refetch()}
          />
        ) : null}

        {step === "params" && selectedTemplate ? (
          <div className="wizard-panel">
            <header className="wizard-panel-header">
              <h3>{selectedTemplate.name}</h3>
              <p>{selectedTemplate.description}</p>
              <p className="muted-inline">场景：{selectedTemplate.scenario}</p>
            </header>
            <form
              onSubmit={(event) => {
                event.preventDefault();
                void goPreview();
              }}
            >
              <TemplateParamFields
                params={selectedTemplate.params}
                control={form.control}
                errors={form.formState.errors}
                disabled={busy}
              />
              <div className="form-actions">
                <Button
                  type="button"
                  variant="subtle"
                  disabled={busy}
                  onClick={() => setStep(prevWizardStep(step))}
                >
                  <ArrowLeft aria-hidden="true" data-icon="inline-start" />
                  上一步
                </Button>
                <Button type="submit" variant="primary" disabled={busy}>
                  预览命令
                  <ArrowRight aria-hidden="true" data-icon="inline-end" />
                </Button>
              </div>
            </form>
          </div>
        ) : null}

        {step === "preview" && draft ? (
          <div className="wizard-panel">
            <header className="wizard-panel-header">
              <h3>确认任务草稿</h3>
              <p>
                模板 <code>{draft.templateId}</code>
                。命令预览中的敏感值已由服务端掩码，前端不会解掩码。
              </p>
            </header>
            <dl className="settings-list">
              <div className="settings-list-item">
                <dt>任务名称</dt>
                <dd>{draft.name}</dd>
              </div>
              <div className="settings-list-item">
                <dt>Cron</dt>
                <dd>
                  {draft.cronExpression}（{draft.timezone}）
                </dd>
              </div>
              <div className="settings-list-item">
                <dt>启用</dt>
                <dd>{draft.enabled ? "是" : "否（草稿默认关闭）"}</dd>
              </div>
            </dl>
            <div className="command-preview">
              <div className="panel-actions">
                <strong>命令预览</strong>
                <Button
                  type="button"
                  variant="subtle"
                  onClick={() => void copyPreview()}
                >
                  <ClipboardCopy aria-hidden="true" data-icon="inline-start" />
                  {copyDone ? "已复制" : "复制"}
                </Button>
              </div>
              <pre className="command-preview-body">{draft.commandPreview}</pre>
            </div>
            <div className="form-actions">
              <Button
                type="button"
                variant="subtle"
                disabled={busy}
                onClick={() => setStep(prevWizardStep(step))}
              >
                <ArrowLeft aria-hidden="true" data-icon="inline-start" />
                返回修改参数
              </Button>
              <Button
                type="button"
                variant="primary"
                disabled={busy}
                onClick={() => void submitDraft()}
              >
                <Check aria-hidden="true" data-icon="inline-start" />
                创建任务
              </Button>
            </div>
          </div>
        ) : null}
      </section>

      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>放弃未保存的向导内容？</AlertDialogTitle>
          <AlertDialogDescription>
            返回任务列表将丢失当前已选模板与已填参数。
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>继续编辑</AlertDialogCancel>
          <AlertDialogAction
            onClick={() => {
              void navigate({ to: "/tasks" });
            }}
          >
            离开向导
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

type SelectStepProps = {
  loading: boolean;
  error: boolean;
  traceId: string | null;
  templates: TemplateDefinition[];
  selectedId: string | null;
  onSelect: (template: TemplateDefinition) => void;
  onRetry: () => void;
};

/**
 * 模板选择步骤。
 *
 * @param props - 列表状态与选择回调。
 * @returns 选择 UI。
 */
function SelectStep({
  loading,
  error,
  traceId,
  templates,
  selectedId,
  onSelect,
  onRetry,
}: SelectStepProps) {
  if (loading) {
    return (
      <div className="empty-state">
        <p>正在加载备份模板…</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="empty-state error">
        <p>模板列表加载失败，请检查 API 服务或登录状态。</p>
        {traceId ? (
          <p className="trace-id">
            traceId: <code>{traceId}</code>
          </p>
        ) : null}
        <Button type="button" variant="subtle" onClick={onRetry}>
          重试
        </Button>
      </div>
    );
  }

  if (templates.length === 0) {
    return (
      <div className="empty-state">
        <p>暂无可用的备份模板。</p>
        <p>请确认后端已注册模板，或稍后再试。</p>
      </div>
    );
  }

  return (
    <ul className="template-card-list">
      {templates.map((template) => {
        const selected = template.id === selectedId;
        return (
          <li key={template.id}>
            <button
              type="button"
              className={selected ? "template-card selected" : "template-card"}
              onClick={() => onSelect(template)}
            >
              <span className="template-card-icon" aria-hidden="true">
                <LayoutTemplate size={18} />
              </span>
              <span className="template-card-body">
                <strong>{template.name}</strong>
                <span>{template.description}</span>
                <span className="muted-inline">
                  {template.scenario} · runner {template.runnerType}
                </span>
              </span>
              <ArrowRight aria-hidden="true" size={16} />
            </button>
          </li>
        );
      })}
    </ul>
  );
}

/**
 * 按稳定错误码给出中文提示（不依赖 message 做分支）。
 *
 * @param code - ApiClientError.code。
 * @returns 文案。
 */
function errorMessageForCode(code: string): string {
  switch (code) {
    case "TEMPLATE_NOT_FOUND":
      return "模板不存在或已下线。";
    case "TEMPLATE_FIELD_INVALID":
    case "TEMPLATE_FIELD_REQUIRED":
    case "TEMPLATE_FIELD_OUT_OF_RANGE":
    case "TEMPLATE_VALIDATION_FAILED":
      return "参数未通过校验，请检查后重试。";
    case "TEMPLATE_RUNNER_UNSUPPORTED":
      return "模板生成的 runner 不受支持。";
    case "TEMPLATE_UNAVAILABLE":
      return "模板服务暂不可用。";
    case "unauthorized":
      return "登录已失效，请重新登录。";
    default:
      return "操作失败，请稍后重试。";
  }
}
