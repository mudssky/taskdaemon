import type * as React from "react";
import { cn } from "@/lib/utils";

/**
 * 渲染带横向滚动容器的数据表格。
 *
 * @param props - 标准 table 属性和可选 className。
 * @returns 表格元素及其滚动容器。
 */
function Table({ className, ...props }: React.ComponentProps<"table">) {
  return (
    <div
      data-slot="table-container"
      className="relative w-full overflow-x-auto"
    >
      <table
        data-slot="table"
        className={cn(
          "w-full min-w-[760px] caption-bottom border-collapse text-sm",
          className,
        )}
        {...props}
      />
    </div>
  );
}

/**
 * 渲染表格头部。
 *
 * @param props - 标准 thead 属性和可选 className。
 * @returns 表格头部元素。
 */
function TableHeader({ className, ...props }: React.ComponentProps<"thead">) {
  return (
    <thead
      data-slot="table-header"
      className={cn("[&_tr]:border-b [&_tr]:border-border", className)}
      {...props}
    />
  );
}

/**
 * 渲染表格主体。
 *
 * @param props - 标准 tbody 属性和可选 className。
 * @returns 表格主体元素。
 */
function TableBody({ className, ...props }: React.ComponentProps<"tbody">) {
  return (
    <tbody
      data-slot="table-body"
      className={cn("[&_tr:last-child]:border-0", className)}
      {...props}
    />
  );
}

/**
 * 渲染表格页脚。
 *
 * @param props - 标准 tfoot 属性和可选 className。
 * @returns 表格页脚元素。
 */
function TableFooter({ className, ...props }: React.ComponentProps<"tfoot">) {
  return (
    <tfoot
      data-slot="table-footer"
      className={cn(
        "border-t bg-muted/50 font-medium [&>tr]:last:border-b-0",
        className,
      )}
      {...props}
    />
  );
}

/**
 * 渲染表格行。
 *
 * @param props - 标准 tr 属性和可选 className。
 * @returns 表格行元素。
 */
function TableRow({ className, ...props }: React.ComponentProps<"tr">) {
  return (
    <tr
      data-slot="table-row"
      className={cn(
        "border-b border-border align-top transition-colors data-[selected=true]:bg-accent",
        className,
      )}
      {...props}
    />
  );
}

/**
 * 渲染表格列头单元格。
 *
 * @param props - 标准 th 属性和可选 className。
 * @returns 表格列头元素。
 */
function TableHead({ className, ...props }: React.ComponentProps<"th">) {
  return (
    <th
      data-slot="table-head"
      className={cn(
        "px-2 py-2.5 text-left align-top text-xs font-bold text-muted-foreground uppercase whitespace-nowrap",
        className,
      )}
      {...props}
    />
  );
}

/**
 * 渲染表格内容单元格。
 *
 * @param props - 标准 td 属性和可选 className。
 * @returns 表格单元格元素。
 */
function TableCell({ className, ...props }: React.ComponentProps<"td">) {
  return (
    <td
      data-slot="table-cell"
      className={cn("px-2 py-2.5 align-top", className)}
      {...props}
    />
  );
}

/**
 * 渲染表格说明。
 *
 * @param props - 标准 caption 属性和可选 className。
 * @returns 表格说明元素。
 */
function TableCaption({
  className,
  ...props
}: React.ComponentProps<"caption">) {
  return (
    <caption
      data-slot="table-caption"
      className={cn("mt-4 text-sm text-muted-foreground", className)}
      {...props}
    />
  );
}

export {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableFooter,
  TableHead,
  TableHeader,
  TableRow,
};
