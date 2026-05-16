import * as AlertDialogPrimitive from "@radix-ui/react-alert-dialog";
import type * as React from "react";
import { cn } from "@/lib/utils";
import { buttonVariants } from "./button";

/**
 * 渲染 AlertDialog 根组件。
 *
 * @param props - Radix AlertDialog Root 属性。
 * @returns AlertDialog 根组件。
 */
function AlertDialog({
  ...props
}: React.ComponentProps<typeof AlertDialogPrimitive.Root>) {
  return <AlertDialogPrimitive.Root data-slot="alert-dialog" {...props} />;
}

/**
 * 渲染 AlertDialog 触发器。
 *
 * @param props - Radix AlertDialog Trigger 属性。
 * @returns AlertDialog 触发器元素。
 */
function AlertDialogTrigger({
  ...props
}: React.ComponentProps<typeof AlertDialogPrimitive.Trigger>) {
  return (
    <AlertDialogPrimitive.Trigger data-slot="alert-dialog-trigger" {...props} />
  );
}

/**
 * 渲染 AlertDialog 门户。
 *
 * @param props - Radix AlertDialog Portal 属性。
 * @returns AlertDialog 门户元素。
 */
function AlertDialogPortal({
  ...props
}: React.ComponentProps<typeof AlertDialogPrimitive.Portal>) {
  return (
    <AlertDialogPrimitive.Portal data-slot="alert-dialog-portal" {...props} />
  );
}

/**
 * 渲染 AlertDialog 遮罩层。
 *
 * @param props - Radix AlertDialog Overlay 属性和可选 className。
 * @returns AlertDialog 遮罩层元素。
 */
function AlertDialogOverlay({
  className,
  ...props
}: React.ComponentProps<typeof AlertDialogPrimitive.Overlay>) {
  return (
    <AlertDialogPrimitive.Overlay
      data-slot="alert-dialog-overlay"
      className={cn(
        "fixed inset-0 z-50 bg-black/35 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:animate-in data-[state=open]:fade-in-0",
        className,
      )}
      {...props}
    />
  );
}

/**
 * 渲染 AlertDialog 内容区域。
 *
 * @param props - Radix AlertDialog Content 属性和可选 className。
 * @returns AlertDialog 内容元素。
 */
function AlertDialogContent({
  className,
  ...props
}: React.ComponentProps<typeof AlertDialogPrimitive.Content>) {
  return (
    <AlertDialogPortal>
      <AlertDialogOverlay />
      <AlertDialogPrimitive.Content
        data-slot="alert-dialog-content"
        className={cn(
          "fixed top-1/2 left-1/2 z-50 grid w-[min(92vw,420px)] -translate-x-1/2 -translate-y-1/2 gap-4 rounded-lg border border-border bg-card p-5 text-foreground shadow-lg data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95 data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95",
          className,
        )}
        {...props}
      />
    </AlertDialogPortal>
  );
}

/**
 * 渲染 AlertDialog 头部。
 *
 * @param props - 标准 div 属性和可选 className。
 * @returns AlertDialog 头部元素。
 */
function AlertDialogHeader({
  className,
  ...props
}: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="alert-dialog-header"
      className={cn("grid gap-2", className)}
      {...props}
    />
  );
}

/**
 * 渲染 AlertDialog 底部操作区。
 *
 * @param props - 标准 div 属性和可选 className。
 * @returns AlertDialog 底部元素。
 */
function AlertDialogFooter({
  className,
  ...props
}: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="alert-dialog-footer"
      className={cn("flex justify-end gap-2", className)}
      {...props}
    />
  );
}

/**
 * 渲染 AlertDialog 标题。
 *
 * @param props - Radix AlertDialog Title 属性和可选 className。
 * @returns AlertDialog 标题元素。
 */
function AlertDialogTitle({
  className,
  ...props
}: React.ComponentProps<typeof AlertDialogPrimitive.Title>) {
  return (
    <AlertDialogPrimitive.Title
      data-slot="alert-dialog-title"
      className={cn("text-base font-bold text-foreground", className)}
      {...props}
    />
  );
}

/**
 * 渲染 AlertDialog 描述。
 *
 * @param props - Radix AlertDialog Description 属性和可选 className。
 * @returns AlertDialog 描述元素。
 */
function AlertDialogDescription({
  className,
  ...props
}: React.ComponentProps<typeof AlertDialogPrimitive.Description>) {
  return (
    <AlertDialogPrimitive.Description
      data-slot="alert-dialog-description"
      className={cn("text-sm text-muted-foreground", className)}
      {...props}
    />
  );
}

/**
 * 渲染 AlertDialog 确认按钮。
 *
 * @param props - Radix AlertDialog Action 属性和可选 className。
 * @returns AlertDialog 确认按钮元素。
 */
function AlertDialogAction({
  className,
  ...props
}: React.ComponentProps<typeof AlertDialogPrimitive.Action>) {
  return (
    <AlertDialogPrimitive.Action
      data-slot="alert-dialog-action"
      className={cn(buttonVariants({ variant: "destructive" }), className)}
      {...props}
    />
  );
}

/**
 * 渲染 AlertDialog 取消按钮。
 *
 * @param props - Radix AlertDialog Cancel 属性和可选 className。
 * @returns AlertDialog 取消按钮元素。
 */
function AlertDialogCancel({
  className,
  ...props
}: React.ComponentProps<typeof AlertDialogPrimitive.Cancel>) {
  return (
    <AlertDialogPrimitive.Cancel
      data-slot="alert-dialog-cancel"
      className={cn(buttonVariants({ variant: "subtle" }), className)}
      {...props}
    />
  );
}

export {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogOverlay,
  AlertDialogPortal,
  AlertDialogTitle,
  AlertDialogTrigger,
};
