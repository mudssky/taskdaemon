import type { RefCallback } from "react";
import { type Control, Controller, type FieldErrors } from "react-hook-form";
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
import type { TemplateParamDef } from "../../../lib/api/types";
import type { TemplateParamValues } from "./template.schema";

type TemplateParamFieldsProps = {
  params: TemplateParamDef[];
  control: Control<TemplateParamValues>;
  errors: FieldErrors<TemplateParamValues>;
  disabled?: boolean;
};

/**
 * 按参数定义动态渲染表单控件；不硬编码具体模板字段。
 *
 * @param props.params - T7a 参数定义。
 * @param props.control - RHF control。
 * @param props.errors - 字段错误。
 * @param props.disabled - 是否禁用。
 * @returns 字段列表。
 */
export function TemplateParamFields({
  params,
  control,
  errors,
  disabled = false,
}: TemplateParamFieldsProps) {
  return (
    <div className="form-grid">
      {params.map((param) => {
        const errorMessage = errorText(errors[param.name]);
        const requiredMark = param.required ? (
          <span className="required-mark" aria-hidden="true">
            *
          </span>
        ) : null;
        const help =
          param.help ||
          (param.type === "secret_ref"
            ? "填写宿主环境变量名，不要填写密钥明文"
            : undefined);

        return (
          <div key={param.name} className="form-field">
            <Label htmlFor={`param-${param.name}`}>
              {param.name}
              {requiredMark}
              {param.sensitive || param.type === "secret_ref" ? (
                <span className="muted-inline"> · 敏感</span>
              ) : null}
            </Label>
            {help ? <p className="field-help">{help}</p> : null}
            <Controller
              name={param.name}
              control={control}
              render={({ field }) =>
                renderControl(param, field, disabled, errorMessage)
              }
            />
            {errorMessage ? (
              <span className="field-error">{errorMessage}</span>
            ) : null}
          </div>
        );
      })}
    </div>
  );
}

/**
 * 渲染单个参数控件。
 *
 * @param param - 参数定义。
 * @param field - RHF field。
 * @param disabled - 禁用。
 * @param errorMessage - 错误文案（用于 aria）。
 * @returns 控件节点。
 */
function renderControl(
  param: TemplateParamDef,
  field: {
    value: unknown;
    onChange: (value: unknown) => void;
    onBlur: () => void;
    name: string;
    ref: RefCallback<HTMLElement>;
  },
  disabled: boolean,
  errorMessage: string | undefined,
) {
  const id = `param-${param.name}`;
  const invalid = Boolean(errorMessage);

  if (param.type === "boolean") {
    return (
      <Label className="checkbox-row">
        <Checkbox
          checked={Boolean(field.value)}
          disabled={disabled}
          onCheckedChange={(checked) => field.onChange(checked === true)}
          aria-invalid={invalid}
        />
        <span>启用</span>
      </Label>
    );
  }

  if (param.type === "enum") {
    const options = param.enumOptions ?? [];
    return (
      <Select
        value={String(field.value ?? "")}
        disabled={disabled || options.length === 0}
        onValueChange={(value) => field.onChange(value)}
      >
        <SelectTrigger id={id} aria-invalid={invalid}>
          <SelectValue placeholder="请选择" />
        </SelectTrigger>
        <SelectContent>
          <SelectGroup>
            {options.map((option) => (
              <SelectItem key={option} value={option}>
                {option}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>
    );
  }

  if (param.type === "number") {
    return (
      <Input
        id={id}
        type="number"
        disabled={disabled}
        value={
          field.value === undefined || field.value === null
            ? ""
            : String(field.value)
        }
        onBlur={field.onBlur}
        onChange={(event) => {
          const raw = event.target.value;
          field.onChange(raw === "" ? "" : Number(raw));
        }}
        aria-invalid={invalid}
        min={param.min}
        max={param.max}
      />
    );
  }

  return (
    <Input
      id={id}
      type="text"
      disabled={disabled}
      value={String(field.value ?? "")}
      onBlur={field.onBlur}
      onChange={(event) => field.onChange(event.target.value)}
      aria-invalid={invalid}
      autoComplete={param.type === "secret_ref" ? "off" : undefined}
      spellCheck={param.type === "secret_ref" ? false : undefined}
    />
  );
}

/**
 * 从 RHF 错误节点提取可读文案。
 *
 * @param error - FieldError 或嵌套。
 * @returns 文案。
 */
function errorText(error: unknown): string | undefined {
  if (!error || typeof error !== "object") {
    return undefined;
  }
  if ("message" in error && typeof error.message === "string") {
    return error.message;
  }
  return undefined;
}
