import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import type * as React from "react";
import { cn } from "@/lib/utils";

const badgeVariants = cva(
  "inline-flex w-fit shrink-0 items-center justify-center gap-1 rounded-full border px-2 py-[3px] text-xs font-bold whitespace-nowrap transition-colors focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/20 [&>svg]:pointer-events-none [&>svg]:size-3",
  {
    variants: {
      variant: {
        default: "border-border bg-muted text-muted-foreground",
        muted: "border-border bg-muted text-muted-foreground",
        success: "border-success-border bg-success text-success-foreground",
        running: "border-success-border bg-success text-success-foreground",
        destructive:
          "border-destructive-border bg-destructive-soft text-destructive-soft-foreground",
        failed:
          "border-destructive-border bg-destructive-soft text-destructive-soft-foreground",
        timeout:
          "border-destructive-border bg-destructive-soft text-destructive-soft-foreground",
        cancelled: "border-warning-border bg-warning text-warning-foreground",
        skipped: "border-warning-border bg-warning text-warning-foreground",
        outline: "border-border bg-card text-foreground",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  },
);

type BadgeProps = React.ComponentProps<"span"> &
  VariantProps<typeof badgeVariants> & { asChild?: boolean };

/**
 * 渲染任务状态等短文本标记。
 *
 * @param props - 标准 span 属性、状态 variant 和 asChild 组合选项。
 * @returns 标记元素或 Slot 包装后的标记内容。
 */
function Badge({
  className,
  variant = "default",
  asChild = false,
  ...props
}: BadgeProps) {
  const Comp = asChild ? Slot : "span";

  return (
    <Comp
      data-slot="badge"
      data-variant={variant}
      className={cn(badgeVariants({ variant }), className)}
      {...props}
    />
  );
}

export { Badge, badgeVariants };
