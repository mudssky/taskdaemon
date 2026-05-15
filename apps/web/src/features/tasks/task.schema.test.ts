import { describe, expect, it } from "vitest";
import {
  defaultTaskFormValues,
  parseEnvText,
  taskFormSchema,
  toTaskPayload,
} from "./task.schema";

describe("task schema helpers", () => {
  it("maps TypeScript script runner to payload with default timeout", () => {
    const values = defaultTaskFormValues(null);
    values.name = "build";
    values.runnerType = "typescript";
    values.commandMode = "script";
    values.scriptPath = "jobs/build.ts";
    values.inline = "";
    values.argsText = "--full --quiet";
    values.envText = "NODE_ENV=production";

    const payload = toTaskPayload(values);

    expect(payload.runner.type).toBe("typescript");
    expect(payload.runner.scriptPath).toBe("jobs/build.ts");
    expect(payload.runner.inline).toBe("");
    expect(payload.runner.args).toEqual(["--full", "--quiet"]);
    expect(payload.runner.env).toEqual({ NODE_ENV: "production" });
    expect(payload.runner.timeoutSeconds).toBe(3600);
  });

  it("requires explicit confirmation for high frequency cron", () => {
    const result = taskFormSchema.safeParse({
      ...defaultTaskFormValues(null),
      name: "fast",
      cronExpression: "*/5 * * * * *",
      inline: "echo ok",
      confirmCronWarnings: false,
    });

    expect(result.success).toBe(false);
    if (!result.success) {
      expect(
        result.error.issues.some((issue) =>
          issue.path.includes("confirmCronWarnings"),
        ),
      ).toBe(true);
    }
  });

  it("parses env text and ignores malformed lines", () => {
    expect(parseEnvText("A=1\nINVALID\nB=hello=world")).toEqual({
      A: "1",
      B: "hello=world",
    });
  });
});
