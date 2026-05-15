export type CronValidation =
  | {
      ok: true;
      fields: string[];
      warnings: CronWarning[];
    }
  | {
      ok: false;
      error: string;
    };

export type CronWarning = "second_level_cron" | "high_frequency_cron";

export function validateCronExpression(
  expression: string,
  timezone: string,
): CronValidation {
  const fields = expression.trim().split(/\s+/).filter(Boolean);
  if (fields.length !== 5 && fields.length !== 6) {
    return { ok: false, error: "cron 表达式必须是 5 或 6 个字段" };
  }
  if (!timezone.trim()) {
    return { ok: false, error: "timezone 不能为空" };
  }
  if (!looksLikeTimezone(timezone)) {
    return { ok: false, error: "timezone 需要使用 Local 或 Area/City 格式" };
  }
  if (fields.some((field) => field.trim() === "")) {
    return { ok: false, error: "cron 字段不能为空" };
  }

  const warnings: CronWarning[] = [];
  const withSeconds = fields.length === 6;
  if (withSeconds) {
    warnings.push("second_level_cron");
  }
  if (isHighFrequency(fields, withSeconds)) {
    warnings.push("high_frequency_cron");
  }
  return { ok: true, fields, warnings };
}

export function warningLabel(warning: CronWarning): string {
  switch (warning) {
    case "second_level_cron":
      return "包含秒字段，任务可能比普通 cron 更频繁触发";
    case "high_frequency_cron":
      return "触发间隔低于 60 秒，请确认机器资源和任务幂等性";
  }
}

function looksLikeTimezone(timezone: string): boolean {
  const value = timezone.trim();
  return value === "Local" || /^[A-Za-z_]+\/[A-Za-z_/-]+$/.test(value);
}

function isHighFrequency(fields: string[], withSeconds: boolean): boolean {
  if (!withSeconds) {
    return false;
  }
  const secondField = fields[0] ?? "";
  if (!secondField.startsWith("*/")) {
    return false;
  }
  const interval = Number.parseInt(secondField.slice(2), 10);
  return Number.isFinite(interval) && interval > 0 && interval < 60;
}
