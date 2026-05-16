import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import type * as React from "react";
import { cn } from "@/lib/utils";

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md border text-sm font-bold transition-colors disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:size-4 [&_svg]:shrink-0",
  {
    variants: {
      variant: {
        default:
          "border-input bg-card text-foreground hover:border-ring hover:bg-accent",
        primary:
          "border-primary bg-primary text-primary-foreground hover:border-primary/90 hover:bg-primary/90",
        destructive:
          "border-destructive bg-destructive text-destructive-foreground hover:bg-destructive/90",
        outline:
          "border-input bg-background text-foreground hover:border-ring hover:bg-accent",
        secondary: "border-secondary bg-secondary text-secondary-foreground",
        subtle:
          "border-input bg-muted text-foreground hover:border-ring hover:bg-accent",
        ghost:
          "border-transparent bg-transparent text-foreground hover:bg-accent",
        link: "h-auto border-transparent bg-transparent p-0 text-primary hover:bg-transparent hover:underline",
      },
      size: {
        default: "h-[34px] px-3",
        sm: "h-8 px-3 text-xs",
        lg: "h-10 px-6",
        icon: "size-[34px] p-0",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  },
);

type ButtonProps = React.ComponentProps<"button"> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean;
  };

/**
 * 渲染 shadcn/ui 风格按钮，支持 variant、size 和 Slot 组合。
 *
 * @param props - 标准 button 属性、variant/size 变体以及 asChild 组合选项。
 * @returns 按钮元素或 Slot 包装后的按钮内容。
 */
function Button({
  className,
  variant,
  size,
  asChild = false,
  ...props
}: ButtonProps) {
  const Comp = asChild ? Slot : "button";

  return (
    <Comp
      className={cn(buttonVariants({ variant, size, className }))}
      data-slot="button"
      {...props}
    />
  );
}

export { Button, buttonVariants };
