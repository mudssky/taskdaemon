/**
 * 通用分页与列表约定（Agent Protocol 子集）。
 */

/** 基于 offset 的分页请求。 */
export type PageQuery = {
  /** 跳过条数，默认 0。 */
  offset?: number;
  /** 返回条数，默认 50，上限 200。 */
  limit?: number;
};

/** 统一分页响应外壳。 */
export type PageResult<T> = {
  items: T[];
  offset: number;
  limit: number;
  /** 是否还有更多；总数未知时可为 undefined。 */
  hasMore: boolean;
  total?: number;
};

/** 资源排序方向。 */
export type SortDirection = "asc" | "desc";

/** Thread / Run 列表排序字段。 */
export type ResourceSort = {
  field: "created_at" | "updated_at";
  direction: SortDirection;
};

/**
 * 规范化分页参数。
 *
 * 参数:
 *   - query: 原始分页查询。
 *
 * 返回值:
 *   - 带默认与上限的 offset/limit。
 */
export function normalizePageQuery(query: PageQuery | undefined): {
  offset: number;
  limit: number;
} {
  const offset = Math.max(0, query?.offset ?? 0);
  const rawLimit = query?.limit ?? 50;
  const limit = Math.min(200, Math.max(1, rawLimit));
  return { offset, limit };
}
