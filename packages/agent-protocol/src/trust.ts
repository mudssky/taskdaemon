/**
 * 信任边界类型（C-4）。
 *
 * 外部网关注入身份；agent-gateway 只消费头、产出 audit/usage 事件。
 * SSO / 多租户管理 / 审计存储 / 配额执行不在本仓库。
 */

/** 注入头规范名称。 */
export const TRUST_HEADERS = {
  subject: "x-auth-subject",
  tenantId: "x-tenant-id",
  traceId: "x-trace-id",
} as const;

export type TrustHeaderName =
  (typeof TRUST_HEADERS)[keyof typeof TRUST_HEADERS];

/** 已解析的调用主体。 */
export type Principal = {
  /** 稳定用户/服务主体 id。 */
  subject: string;
  /** 租户 id；开发期可为 `dev`。 */
  tenantId: string;
  /** 透传或自生成的 trace id。 */
  traceId: string;
  /** 解析来源。 */
  source: "gateway-headers" | "dev-single-principal";
  /** 额外声明（roles 等）；本仓库不解释。 */
  claims?: Record<string, unknown>;
};

/**
 * 从请求解析 Principal。
 * G2 实现；G1 只冻接口。
 */
export interface PrincipalResolver {
  /**
   * 解析主体。
   *
   * 参数:
   *   - headers: 小写键的 HTTP 头映射。
   * 返回值:
   *   - Principal；无法解析时抛出或返回 Result 由实现约定——
   *     契约要求：生产模式缺 subject 必须失败；dev 模式可回落默认主体。
   */
  resolve(headers: Record<string, string | undefined>): Promise<Principal>;
}

/** PrincipalResolver 运行模式。 */
export type PrincipalResolverMode = "dev-single-principal" | "gateway-headers";

/** dev 默认主体（无外部网关时）。 */
export const DEV_SINGLE_PRINCIPAL: Omit<Principal, "traceId"> = {
  subject: "dev-user",
  tenantId: "dev",
  source: "dev-single-principal",
};

/** Audit 事件：谁在何时对哪个 workspace 做了什么。 */
export type AuditEvent = {
  type: "agent.audit";
  eventId: string;
  timestamp: string;
  traceId: string;
  subject: string;
  tenantId: string;
  threadId?: string;
  runId?: string;
  workspaceRoot?: string | null;
  action:
    | "thread.create"
    | "thread.delete"
    | "run.create"
    | "run.cancel"
    | "run.steer"
    | "run.follow_up"
    | "tool.invoke"
    | "session.dispose"
    | "model.switch";
  /** 工具名等；无则省略。 */
  resource?: string;
  outcome: "success" | "denied" | "error";
  message?: string;
  metadata?: Record<string, unknown>;
};

/** Usage 事件：token / 模型成本线索（本仓库只产出，不聚合计费）。 */
export type UsageEvent = {
  type: "agent.usage";
  eventId: string;
  timestamp: string;
  traceId: string;
  subject: string;
  tenantId: string;
  threadId: string;
  runId: string;
  runtimeId: string;
  model?: string;
  inputTokens?: number;
  outputTokens?: number;
  totalTokens?: number;
  /** 原始 cost 字段若 runtime 提供则可填；不做汇率换算。 */
  cost?: number;
  currency?: string;
  metadata?: Record<string, unknown>;
};

/** 事件外送目标抽象（G2 实现；可打日志 / webhook / 外部总线）。 */
export interface TrustEventEmitter {
  /**
   * 发出 audit 事件。
   *
   * 参数:
   *   - event: AuditEvent。
   * 返回值:
   *   - Promise<void>；失败不得阻断主请求（记录本地日志即可）。
   */
  emitAudit(event: AuditEvent): Promise<void>;

  /**
   * 发出 usage 事件。
   *
   * 参数:
   *   - event: UsageEvent。
   * 返回值:
   *   - Promise<void>。
   */
  emitUsage(event: UsageEvent): Promise<void>;
}
