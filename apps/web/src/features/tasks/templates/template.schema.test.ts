import { describe, expect, it } from "vitest";
import type {
  TemplateParamDef,
  TemplateTaskDraft,
} from "../../../lib/api/types";
import {
  areParamValuesDirty,
  buildParamSchema,
  defaultParamValues,
  draftToTaskPayload,
  extractTemplateFieldErrors,
  mapTemplateFieldErrors,
  toRenderParams,
} from "./template.schema";

/** 虚构「第四模板」：覆盖全部六种参数类型，证明无需改前端映射代码。 */
const fictionTemplateParams: TemplateParamDef[] = [
  {
    name: "label",
    type: "string",
    required: true,
    help: "显示名",
  },
  {
    name: "workers",
    type: "number",
    required: true,
    default: 2,
    min: 1,
    max: 8,
    help: "并发",
  },
  {
    name: "dryRun",
    type: "boolean",
    required: false,
    default: true,
    help: "演练",
  },
  {
    name: "mode",
    type: "enum",
    required: true,
    enumOptions: ["fast", "safe"],
    default: "safe",
    help: "模式",
  },
  {
    name: "targetPath",
    type: "path",
    required: true,
    help: "输出路径",
  },
  {
    name: "tokenEnv",
    type: "secret_ref",
    required: false,
    default: "BACKUP_TOKEN",
    sensitive: true,
    help: "令牌环境变量名",
  },
];

describe("template.schema dynamic form", () => {
  it("builds defaults for all param types without template-specific code", () => {
    const defaults = defaultParamValues(fictionTemplateParams);
    expect(defaults).toEqual({
      label: "",
      workers: 2,
      dryRun: true,
      mode: "safe",
      targetPath: "",
      tokenEnv: "BACKUP_TOKEN",
    });
  });

  it("validates required and range rules from ParamDef", () => {
    const schema = buildParamSchema(fictionTemplateParams);
    const ok = schema.safeParse({
      label: "nightly",
      workers: 4,
      dryRun: false,
      mode: "fast",
      targetPath: "/var/backups/x",
      tokenEnv: "BACKUP_TOKEN",
    });
    expect(ok.success).toBe(true);

    const missing = schema.safeParse({
      label: "",
      workers: 4,
      dryRun: false,
      mode: "fast",
      targetPath: "/tmp",
      tokenEnv: "BACKUP_TOKEN",
    });
    expect(missing.success).toBe(false);

    const outOfRange = schema.safeParse({
      label: "x",
      workers: 99,
      dryRun: false,
      mode: "fast",
      targetPath: "/tmp",
      tokenEnv: "BACKUP_TOKEN",
    });
    expect(outOfRange.success).toBe(false);

    const badEnum = schema.safeParse({
      label: "x",
      workers: 2,
      dryRun: false,
      mode: "turbo",
      targetPath: "/tmp",
      tokenEnv: "BACKUP_TOKEN",
    });
    expect(badEnum.success).toBe(false);
  });

  it("maps form values to render params and strips empty optional strings", () => {
    const params = toRenderParams(fictionTemplateParams, {
      label: "  job  ",
      workers: "3",
      dryRun: true,
      mode: "safe",
      targetPath: "/data",
      tokenEnv: "  ",
    });
    expect(params).toEqual({
      label: "job",
      workers: 3,
      dryRun: true,
      mode: "safe",
      targetPath: "/data",
    });
  });

  it("converts draft to createTask payload without preview fields", () => {
    const draft: TemplateTaskDraft = {
      name: "pg-nightly",
      description: "from template",
      enabled: false,
      cronExpression: "30 2 * * *",
      timezone: "Local",
      confirmCronWarnings: false,
      runner: {
        type: "shell",
        inline: "pg_dump -h 'db'",
        args: [],
        env: {},
        timeoutSeconds: 3600,
      },
      commandPreview: "pg_dump -h 'db' # password via env ***",
      templateId: "postgres-pg-dump",
    };
    const payload = draftToTaskPayload(draft);
    expect(payload).toEqual({
      name: "pg-nightly",
      description: "from template",
      enabled: false,
      cronExpression: "30 2 * * *",
      timezone: "Local",
      confirmCronWarnings: false,
      runner: {
        type: "shell",
        inline: "pg_dump -h 'db'",
        args: [],
        env: {},
        timeoutSeconds: 3600,
        scriptPath: undefined,
        workDir: undefined,
        outputLimitBytes: undefined,
      },
    });
    expect(payload).not.toHaveProperty("commandPreview");
    expect(payload).not.toHaveProperty("templateId");
  });

  it("maps server field errors params.* to form field names", () => {
    const fields = extractTemplateFieldErrors({
      fields: [
        {
          path: "params.host",
          reason: "required",
          code: "TEMPLATE_FIELD_REQUIRED",
        },
        {
          path: "params.port",
          reason: "out of range",
          code: "TEMPLATE_FIELD_OUT_OF_RANGE",
        },
      ],
    });
    expect(mapTemplateFieldErrors(fields)).toEqual({
      host: "required",
      port: "out of range",
    });
  });

  it("detects dirty param values against defaults", () => {
    const defaults = defaultParamValues(fictionTemplateParams);
    expect(areParamValuesDirty(fictionTemplateParams, defaults)).toBe(false);
    expect(
      areParamValuesDirty(fictionTemplateParams, {
        ...defaults,
        label: "changed",
      }),
    ).toBe(true);
  });
});
