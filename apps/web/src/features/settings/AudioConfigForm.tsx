import { zodResolver } from "@hookform/resolvers/zod";
import { Copy, Save, ShieldAlert } from "lucide-react";
import { type ReactNode, useEffect, useMemo, useState } from "react";
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
import { ApiClientError } from "../../lib/api/client";
import type {
  AudioConfig,
  ConfigSectionWriteResponse,
} from "../../lib/api/types";
import {
  type AudioConfigFormValues,
  audioConfigFormSchema,
  defaultAudioConfigFormValues,
  isEmptyAudioConfigPayload,
  toAudioConfigWritePayload,
} from "./audio-config.schema";
import {
  audioFileOnlyDisplayItems,
  configFieldErrorMessage,
  createOneTimeTokenScratch,
  extractConfigFieldErrors,
  mapConfigPathToFormField,
  needsPrivateNetworkConfirm,
  needsTokenResetConfirm,
  payloadContainsToken,
} from "./audio-config-errors";
import { usePutConfigSectionMutation } from "./settings.queries";

type AudioConfigFormProps = {
  config: AudioConfig | undefined;
  isLoading: boolean;
  isError: boolean;
  errorTraceId: string | null;
  onRestartRequired: (paths: string[]) => void;
  onRefetch: () => void;
};

type PendingConfirm = "private-network" | "token-reset" | null;

/**
 * 音频 section 可写配置表单（C-1 PUT /api/config/audio）。
 *
 * 参数:
 *   - config: 当前安全配置
 *   - isLoading / isError: 查询状态
 *   - errorTraceId: 加载失败 traceId
 *   - onRestartRequired: 保存后需重启字段回调
 *   - onRefetch: 重新加载
 *
 * 返回值:
 *   - JSX.Element
 */
export function AudioConfigForm({
  config,
  isLoading,
  isError,
  errorTraceId,
  onRestartRequired,
  onRefetch,
}: AudioConfigFormProps) {
  const putMutation = usePutConfigSectionMutation();
  const form = useForm<AudioConfigFormValues>({
    resolver: zodResolver(audioConfigFormSchema),
    defaultValues: defaultAudioConfigFormValues(config),
    mode: "onChange",
  });
  const values = form.watch();
  const [pendingConfirm, setPendingConfirm] = useState<PendingConfirm>(null);
  const [saveResult, setSaveResult] =
    useState<ConfigSectionWriteResponse | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const [formTraceId, setFormTraceId] = useState<string | null>(null);
  const [oneTimeToken, setOneTimeToken] = useState<string | null>(null);
  const [tokenCopied, setTokenCopied] = useState(false);

  useEffect(() => {
    form.reset(defaultAudioConfigFormValues(config));
    setSaveResult(null);
  }, [config, form]);

  useEffect(() => {
    if (!form.formState.isDirty) {
      return;
    }
    const onBeforeUnload = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = "";
    };
    window.addEventListener("beforeunload", onBeforeUnload);
    return () => window.removeEventListener("beforeunload", onBeforeUnload);
  }, [form.formState.isDirty]);

  const fileOnlyItems = useMemo(
    () => (config ? audioFileOnlyDisplayItems(config) : []),
    [config],
  );

  async function submitPayload() {
    const payload = toAudioConfigWritePayload(
      form.getValues(),
      form.formState.dirtyFields,
    );
    if (isEmptyAudioConfigPayload(payload)) {
      return;
    }
    setFormError(null);
    setFormTraceId(null);
    try {
      const result = await putMutation.mutateAsync({
        section: "audio",
        body: payload,
      });
      setSaveResult(result);
      if (result.restartRequired.length > 0) {
        onRestartRequired(result.restartRequired);
      }
      if (payloadContainsToken(payload) && payload.inbound?.token) {
        const scratch = createOneTimeTokenScratch(payload.inbound.token);
        setOneTimeToken(scratch.oneTimeToken);
      } else {
        setOneTimeToken(null);
      }
      form.reset(defaultAudioConfigFormValues(result.config));
    } catch (error) {
      if (error instanceof ApiClientError) {
        setFormTraceId(error.traceId);
        const fieldErrors = extractConfigFieldErrors(error.details);
        for (const fieldError of fieldErrors) {
          const formField = mapConfigPathToFormField(fieldError.path);
          if (formField) {
            form.setError(formField as keyof AudioConfigFormValues, {
              type: "server",
              message: configFieldErrorMessage(fieldError),
            });
          }
        }
        setFormError(
          fieldErrors.length > 0
            ? "部分字段未通过校验，请修正后重试"
            : "配置保存失败，请检查服务状态后重试",
        );
        return;
      }
      setFormError("配置保存失败，请稍后重试");
    }
  }

  function handleSaveClick() {
    void form.handleSubmit(async () => {
      const current = form.getValues();
      if (needsPrivateNetworkConfirm(config, current.allowPrivateNetworks)) {
        setPendingConfirm("private-network");
        return;
      }
      if (needsTokenResetConfirm(current.resetToken, current.token)) {
        setPendingConfirm("token-reset");
        return;
      }
      await submitPayload();
    })();
  }

  async function copyOneTimeToken() {
    if (!oneTimeToken) {
      return;
    }
    await navigator.clipboard.writeText(oneTimeToken);
    setTokenCopied(true);
    window.setTimeout(() => setTokenCopied(false), 1500);
  }

  if (isLoading) {
    return (
      <section className="panel wide" aria-labelledby="audio-config-title">
        <div className="panel-header">
          <div>
            <h2 id="audio-config-title">音频配置</h2>
            <p>正在加载配置…</p>
          </div>
        </div>
      </section>
    );
  }

  if (isError) {
    return (
      <section className="panel wide" aria-labelledby="audio-config-title">
        <div className="panel-header">
          <div>
            <h2 id="audio-config-title">音频配置</h2>
            <p>配置加载失败。请确认已登录且后端可用，然后重试。</p>
          </div>
          <Button type="button" variant="outline" onClick={onRefetch}>
            重试
          </Button>
        </div>
        {errorTraceId ? (
          <p className="muted mono">traceId: {errorTraceId}</p>
        ) : null}
      </section>
    );
  }

  const isBusy = putMutation.isPending || form.formState.isSubmitting;
  const canSave = form.formState.isDirty && !isBusy;

  return (
    <>
      <section className="panel wide" aria-labelledby="audio-config-title">
        <div className="panel-header">
          <div>
            <h2 id="audio-config-title">音频配置</h2>
            <p>
              按 section
              独立保存。热生效字段立即生效；需重启字段会落盘并提示重启。
            </p>
          </div>
          <div className="panel-actions">
            <Button
              type="button"
              variant="outline"
              disabled={isBusy}
              onClick={onRefetch}
            >
              刷新
            </Button>
            <Button type="button" disabled={!canSave} onClick={handleSaveClick}>
              <Save aria-hidden="true" size={16} />
              保存音频配置
            </Button>
          </div>
        </div>

        <form
          className="task-form"
          onSubmit={(event) => {
            event.preventDefault();
            handleSaveClick();
          }}
        >
          <fieldset className="fieldset">
            <legend>热生效</legend>
            <div className="form-grid">
              <div className="check-row span-2">
                <Checkbox
                  checked={values.autoplayEnabled}
                  onCheckedChange={(checked) =>
                    form.setValue("autoplayEnabled", checked === true, {
                      shouldDirty: true,
                      shouldValidate: true,
                    })
                  }
                  id="audio-autoplay-enabled"
                />
                <Label htmlFor="audio-autoplay-enabled">启用自动播放</Label>
              </div>

              <Field id="audio-autoplay-target" label="播放目标">
                <Select
                  value={values.autoplayTarget}
                  onValueChange={(value) =>
                    form.setValue(
                      "autoplayTarget",
                      value === "frontend" ? "frontend" : "backend",
                      { shouldDirty: true, shouldValidate: true },
                    )
                  }
                >
                  <SelectTrigger id="audio-autoplay-target">
                    <SelectValue placeholder="选择目标" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      <SelectItem value="backend">
                        backend（服务端出声）
                      </SelectItem>
                      <SelectItem value="frontend">
                        frontend（浏览器出声）
                      </SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <ErrorMessage
                  message={form.formState.errors.autoplayTarget?.message}
                />
              </Field>

              <Field id="audio-queue-limit" label="队列上限（0=无上限）">
                <Input
                  id="audio-queue-limit"
                  type="number"
                  {...form.register("queueLimit", { valueAsNumber: true })}
                />
                <ErrorMessage
                  message={form.formState.errors.queueLimit?.message}
                />
              </Field>

              <Field id="audio-history-limit" label="历史条数（0=全部）">
                <Input
                  id="audio-history-limit"
                  type="number"
                  {...form.register("historyLimit", { valueAsNumber: true })}
                />
                <ErrorMessage
                  message={form.formState.errors.historyLimit?.message}
                />
              </Field>

              <Field id="audio-max-bytes" label="入站大小上限（字节）">
                <Input
                  id="audio-max-bytes"
                  type="number"
                  {...form.register("maxBytes", { valueAsNumber: true })}
                />
                <ErrorMessage
                  message={form.formState.errors.maxBytes?.message}
                />
              </Field>

              <Field id="audio-schemes" label="允许协议（逗号分隔）">
                <Input
                  id="audio-schemes"
                  className="mono"
                  {...form.register("allowedSchemesText")}
                  placeholder="https, http"
                />
                <ErrorMessage
                  message={form.formState.errors.allowedSchemesText?.message}
                />
              </Field>

              <Field id="audio-hosts" label="允许 Host（空=不限制）">
                <Input
                  id="audio-hosts"
                  className="mono"
                  {...form.register("allowedHostsText")}
                  placeholder="example.com, cdn.example.com"
                />
                <ErrorMessage
                  message={form.formState.errors.allowedHostsText?.message}
                />
              </Field>

              <Field id="audio-download-timeout" label="下载超时（秒）">
                <Input
                  id="audio-download-timeout"
                  type="number"
                  {...form.register("downloadTimeoutSeconds", {
                    valueAsNumber: true,
                  })}
                />
                <ErrorMessage
                  message={
                    form.formState.errors.downloadTimeoutSeconds?.message
                  }
                />
              </Field>

              <Field id="audio-max-redirects" label="最大重定向">
                <Input
                  id="audio-max-redirects"
                  type="number"
                  {...form.register("maxRedirects", { valueAsNumber: true })}
                />
                <ErrorMessage
                  message={form.formState.errors.maxRedirects?.message}
                />
              </Field>

              <div className="check-row span-2">
                <Checkbox
                  checked={values.allowPrivateNetworks}
                  onCheckedChange={(checked) =>
                    form.setValue("allowPrivateNetworks", checked === true, {
                      shouldDirty: true,
                      shouldValidate: true,
                    })
                  }
                  id="audio-private-networks"
                />
                <Label htmlFor="audio-private-networks">
                  允许私网地址（危险：扩大 SSRF 面）
                </Label>
              </div>
            </div>
          </fieldset>

          <fieldset className="fieldset">
            <legend>入站 Token（敏感）</legend>
            <div className="settings-list-item">
              <p className="eyebrow">当前状态</p>
              <strong>
                {config?.inbound.tokenConfigured ? "已配置" : "未配置"}
              </strong>
              <span className="muted">
                明文永不回显；仅展示是否已配置 hash。
              </span>
            </div>
            <div className="check-row">
              <Checkbox
                checked={values.resetToken}
                onCheckedChange={(checked) => {
                  const next = checked === true;
                  form.setValue("resetToken", next, {
                    shouldDirty: true,
                    shouldValidate: true,
                  });
                  if (!next) {
                    form.setValue("token", "", {
                      shouldDirty: true,
                      shouldValidate: true,
                    });
                  }
                }}
                id="audio-reset-token"
              />
              <Label htmlFor="audio-reset-token">重新设置 Token</Label>
            </div>
            {values.resetToken ? (
              <Field id="audio-token" label="新 Token（提交后只显示一次）">
                <Input
                  id="audio-token"
                  type="password"
                  autoComplete="new-password"
                  {...form.register("token")}
                  placeholder="输入新的 Bearer Token"
                />
                <ErrorMessage message={form.formState.errors.token?.message} />
              </Field>
            ) : null}
            {oneTimeToken ? (
              <div className="token-once-box" role="status">
                <div>
                  <strong>新 Token 仅此一次可见</strong>
                  <p className="mono token-once-value">{oneTimeToken}</p>
                  <p className="muted">关闭或刷新后将无法再次查看明文。</p>
                </div>
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => {
                    void copyOneTimeToken();
                  }}
                >
                  <Copy aria-hidden="true" size={16} />
                  {tokenCopied ? "已复制" : "复制"}
                </Button>
              </div>
            ) : null}
          </fieldset>

          <fieldset className="fieldset restart-fieldset">
            <legend>需重启后生效</legend>
            <p className="muted">
              保存后会写入配置文件，但当前进程仍使用旧值，直到重启 taskdaemon。
            </p>
            <Field
              id="audio-transcode-timeout"
              label="转码超时（秒）· ffmpeg.transcodeTimeoutSeconds"
            >
              <Input
                id="audio-transcode-timeout"
                type="number"
                {...form.register("transcodeTimeoutSeconds", {
                  valueAsNumber: true,
                })}
              />
              <ErrorMessage
                message={form.formState.errors.transcodeTimeoutSeconds?.message}
              />
            </Field>
          </fieldset>

          <fieldset className="fieldset">
            <legend>仅配置文件（只读）</legend>
            <p className="muted">
              下列项只能编辑配置文件（taskdaemon.yaml /
              local），页面不提供输入框。
            </p>
            <dl className="settings-list">
              {fileOnlyItems.map((item) => (
                <div key={item.path} className="settings-list-item">
                  <dt>
                    {item.label}
                    <span className="muted mono"> · {item.path}</span>
                  </dt>
                  <dd>{item.status}</dd>
                </div>
              ))}
            </dl>
          </fieldset>

          {formError ? (
            <div className="form-error-box" role="alert">
              <p className="form-error">{formError}</p>
              {formTraceId ? (
                <p className="muted mono">traceId: {formTraceId}</p>
              ) : null}
            </div>
          ) : null}

          {saveResult ? (
            <div className="save-result-box" role="status">
              {saveResult.restartRequired.length === 0 ? (
                <p>
                  <strong>已生效</strong>
                  {saveResult.applied.length > 0
                    ? `：${saveResult.applied.join(", ")}`
                    : ""}
                </p>
              ) : (
                <p>
                  <strong>已落盘，部分项需重启</strong>：
                  {saveResult.restartRequired.join(", ")}
                </p>
              )}
              {saveResult.reload.subsystems.length > 0 ? (
                <ul className="subsystem-list">
                  {saveResult.reload.subsystems.map((subsystem) => (
                    <li key={subsystem.name}>
                      {subsystem.name}: {subsystem.status}
                      {subsystem.error ? `（${subsystem.error}）` : ""}
                    </li>
                  ))}
                </ul>
              ) : null}
            </div>
          ) : null}
        </form>
      </section>

      <AlertDialog
        open={pendingConfirm !== null}
        onOpenChange={(open) => {
          if (!open) {
            setPendingConfirm(null);
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {pendingConfirm === "private-network"
                ? "确认允许私网地址？"
                : "确认重置入站 Token？"}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {pendingConfirm === "private-network" ? (
                <>
                  开启后入站 URL 可访问私网地址，可能扩大 SSRF
                  风险。请确认运行环境可信且策略可接受。
                </>
              ) : (
                <>
                  新 Token 生效后，仍使用旧 Bearer
                  的调用方会立即鉴权失败。请同步更新所有客户端。
                </>
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                setPendingConfirm(null);
                void submitPayload();
              }}
            >
              <ShieldAlert aria-hidden="true" size={16} />
              确认保存
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}

function Field({
  id,
  label,
  children,
}: {
  id: string;
  label: string;
  children: ReactNode;
}) {
  return (
    <div className="field">
      <Label htmlFor={id}>{label}</Label>
      {children}
    </div>
  );
}

function ErrorMessage({ message }: { message?: string }) {
  return message ? <span className="field-error">{message}</span> : null;
}
