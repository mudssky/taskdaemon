import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

/**
 * 合并条件 className，并按 Tailwind 优先级消解冲突。
 *
 * 参数:
 *   - inputs: clsx 支持的 className 输入列表。
 *
 * 返回值:
 *   - 合并后的 className 字符串。
 */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
