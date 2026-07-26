import { z } from "zod";
import {
  type RunnerType,
  runnerTypes,
  type TaskPayload,
  type TemplateParamDef,
  type TemplateParamType,
  type TemplateTaskDraft,
  templateParamTypes,
} from "../../../lib/api/types";

/** 表单值：键为参数 name，值为用户输入。 */
export type TemplateParamValues = Record<string, unknown>;

/**
 * 判断字符串是否为合法模板参数类型。
 *
 * @param value - 待检查值。
 * @returns 是否属于 T7a 枚举。
 */
export function isTemplateParamType(value: string): value is TemplateParamType {
  return (templateParamTypes as readonly string[]).includes(value);
}

/**
 * 从参数定义生成默认表单值（动态，不按模板 id 硬编码）。
 *
 * @param params - 模板参数定义列表。
 * @returns name → 默认值映射。
 */
export function defaultParamValues(
  params: TemplateParamDef[],
): TemplateParamValues {
  const values: TemplateParamValues = {};
  for (const param of params) {
    if (param.default !== undefined && param.default !== null) {
      values[param.name] = param.default;
      continue;
    }
    switch (param.type) {
      case "boolean":
        values[param.name] = false;
        break;
      case "number":
        values[param.name] = param.min ?? 0;
        break;
      case "enum":
        values[param.name] = param.enumOptions?.[0] ?? "";
        break;
      default:
        values[param.name] = "";
        break;
    }
  }
  return values;
}

/**
 * 为单个参数构建 Zod schema 片段（客户端规则不宽于服务端）。
 *
 * @param param - 参数定义。
 * @returns Zod schema。
 */
function buildSingleParamSchema(param: TemplateParamDef): z.ZodType {
  switch (param.type) {
    case "number": {
      let schema = z.coerce.number({
        error: "必须是数字",
      });
      if (param.min !== undefined) {
        schema = schema.min(param.min, `不能小于 ${param.min}`);
      }
      if (param.max !== undefined) {
        schema = schema.max(param.max, `不能大于 ${param.max}`);
      }
      if (!param.required) {
        return schema.optional();
      }
      return schema;
    }
    case "boolean":
      return z.boolean();
    case "enum": {
      const options = param.enumOptions ?? [];
      if (options.length === 0) {
        const emptyEnum = z.string();
        return param.required
          ? emptyEnum.min(1, "请选择选项")
          : emptyEnum.optional();
      }
      const enumSchema = z.enum(options as [string, ...string[]], {
        error: "请选择合法选项",
      });
      return param.required ? enumSchema : enumSchema.optional();
    }
    case "string":
    case "path":
    case "secret_ref": {
      const base = z.string();
      if (param.required) {
        return base.trim().min(1, "不能为空");
      }
      return base;
    }
    default: {
      const _exhaustive: never = param.type;
      return z.unknown();
    }
  }
}

/**
 * 根据参数定义列表动态生成 Zod object schema。
 * 新增后端模板时无需改本函数逻辑。
 *
 * @param params - 参数定义。
 * @returns Zod object。
 */
export function buildParamSchema(
  params: TemplateParamDef[],
): z.ZodObject<Record<string, z.ZodType>> {
  const shape: Record<string, z.ZodType> = {};
  for (const param of params) {
    shape[param.name] = buildSingleParamSchema(param);
  }
  return z.object(shape);
}

/**
 * 将表单值裁剪为 render API 的 params 对象。
 *
 * @param params - 参数定义（用于类型归一）。
 * @param values - 表单值。
 * @returns render body.params。
 */
export function toRenderParams(
  params: TemplateParamDef[],
  values: TemplateParamValues,
): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const param of params) {
    const raw = values[param.name];
    if (raw === undefined) {
      continue;
    }
    if (param.type === "number") {
      if (raw === "" || raw === null) {
        continue;
      }
      const n = typeof raw === "number" ? raw : Number(raw);
      if (!Number.isNaN(n)) {
        out[param.name] = n;
      }
      continue;
    }
    if (param.type === "boolean") {
      out[param.name] = Boolean(raw);
      continue;
    }
    if (typeof raw === "string") {
      const trimmed = raw.trim();
      if (!param.required && trimmed === "") {
        continue;
      }
      out[param.name] = trimmed;
      continue;
    }
    out[param.name] = raw;
  }
  return out;
}

/**
 * 将渲染草稿转为既有 createTask payload（剥离 preview / templateId）。
 *
 * @param draft - T7a 渲染结果。
 * @returns TaskPayload。
 */
export function draftToTaskPayload(draft: TemplateTaskDraft): TaskPayload {
  const runnerType = (runnerTypes as readonly string[]).includes(
    draft.runner.type,
  )
    ? (draft.runner.type as RunnerType)
    : ("shell" as RunnerType);

  return {
    name: draft.name,
    description: draft.description,
    enabled: draft.enabled ?? false,
    cronExpression: draft.cronExpression,
    timezone: draft.timezone,
    confirmCronWarnings: draft.confirmCronWarnings,
    runner: {
      type: runnerType,
      inline: draft.runner.inline,
      scriptPath: draft.runner.scriptPath,
      args: draft.runner.args,
      workDir: draft.runner.workDir,
      env: draft.runner.env,
      timeoutSeconds: draft.runner.timeoutSeconds,
      outputLimitBytes: draft.runner.outputLimitBytes,
    },
  };
}

/** 服务端字段级错误项（C-1 / T7a 同形）。 */
export type TemplateFieldError = {
  path: string;
  reason: string;
  code: string;
};

/**
 * 从 ApiClientError.details 提取 fields 列表。
 *
 * @param details - error.details。
 * @returns 字段错误数组。
 */
export function extractTemplateFieldErrors(
  details: unknown,
): TemplateFieldError[] {
  if (!details || typeof details !== "object") {
    return [];
  }
  if (!("fields" in details) || !Array.isArray(details.fields)) {
    return [];
  }
  const result: TemplateFieldError[] = [];
  for (const item of details.fields) {
    if (!item || typeof item !== "object") {
      continue;
    }
    if (!("path" in item) || !("reason" in item)) {
      continue;
    }
    if (typeof item.path !== "string" || typeof item.reason !== "string") {
      continue;
    }
    const code =
      "code" in item && typeof item.code === "string" ? item.code : "";
    result.push({
      path: item.path,
      reason: item.reason,
      code,
    });
  }
  return result;
}

/**
 * 将 `params.host` 形式 path 映射为表单字段名 → 文案。
 *
 * @param fields - 服务端 fields。
 * @returns 字段名 → reason。
 */
export function mapTemplateFieldErrors(
  fields: TemplateFieldError[],
): Record<string, string> {
  const map: Record<string, string> = {};
  for (const field of fields) {
    const name = field.path.startsWith("params.")
      ? field.path.slice("params.".length)
      : field.path;
    if (name && !map[name]) {
      map[name] = field.reason;
    }
  }
  return map;
}

/**
 * 判断参数值相对默认值是否已修改（用于 dirty / 未保存提示）。
 *
 * @param params - 参数定义。
 * @param values - 当前值。
 * @returns 是否 dirty。
 */
export function areParamValuesDirty(
  params: TemplateParamDef[],
  values: TemplateParamValues,
): boolean {
  const defaults = defaultParamValues(params);
  for (const param of params) {
    const current = values[param.name];
    const base = defaults[param.name];
    if (param.type === "number") {
      if (Number(current) !== Number(base)) {
        return true;
      }
      continue;
    }
    if (param.type === "boolean") {
      if (Boolean(current) !== Boolean(base)) {
        return true;
      }
      continue;
    }
    if (String(current ?? "") !== String(base ?? "")) {
      return true;
    }
  }
  return false;
}
