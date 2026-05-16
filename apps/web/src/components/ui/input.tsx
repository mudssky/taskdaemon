import type * as React from "react";
import { cn } from "@/lib/utils";

/**
 * 渲染与当前表单视觉对齐的文本输入框。
 *
 * @param props - 标准 input 属性和可选 className。
 * @returns 输入框元素。
 */
function Input({ className, type, ...props }: React.ComponentProps<"input">) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        "h-[34px] w-full min-w-0 rounded-md border border-input bg-card px-2.5 py-2 text-sm text-foreground outline-none transition-colors placeholder:text-muted-foreground disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50",
        "focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/20",
        "aria-invalid:border-destructive aria-invalid:ring-2 aria-invalid:ring-destructive/20",
        className,
      )}
      {...props}
    />
  );
}

export { Input };
