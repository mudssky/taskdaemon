/**
 * 绑定一次 stream run 到 StreamViewState（G5：续传 / steer / HITL）。
 */

import type {
  AgentMessage,
  AguiEvent,
  CreateRunRequest,
  HitlDecision,
  RuntimeCapabilities,
} from "@taskdaemon/agent-protocol";
import { useCallback, useEffect, useRef, useState } from "react";
import { getAgentApi } from "../../lib/api/agent-api";
import { AgentApiError } from "../../lib/api/errors";
import { shouldShowThinking, toolCallDetailMode } from "./capability-ui";
import { validateHitlResponse } from "./hitl";
import {
  appendLocalUserMessage,
  appendSteerMessage,
  applyHitlDecision,
  applyLocalCancel,
  createEmptyStreamState,
  hydrateTimelineFromMessages,
  markHitlTimedOut,
  markStreamConnection,
  markStreamDisconnected,
  reduceAguiEvent,
  type StreamViewState,
  toggleThinkingCollapsed,
  toggleToolCollapsed,
} from "./stream-reducer";

type UseRunStreamArgs = {
  threadId: string | undefined;
  capabilities: RuntimeCapabilities | undefined;
  historyMessages: AgentMessage[] | undefined;
};

/**
 * 会话流式发送 / 中断 / 续传 / steering / HITL。
 *
 * 参数:
 *   - args.threadId: 当前会话。
 *   - args.capabilities: 能力声明。
 *   - args.historyMessages: REST 历史。
 *
 * 返回值:
 *   - 视图状态与动作。
 */
export function useRunStream({
  threadId,
  capabilities,
  historyMessages,
}: UseRunStreamArgs) {
  const [state, setState] = useState<StreamViewState>(createEmptyStreamState);
  const streamAbortRef = useRef<(() => void) | null>(null);
  const activeRunIdRef = useRef<string | null>(null);
  const lastEventIdRef = useRef<string | undefined>(undefined);
  const statusRef = useRef(state.status);
  const reconnectAttemptRef = useRef(0);
  const intentionalAbortRef = useRef(false);
  const hitlStartedAtRef = useRef<Map<string, number>>(new Map());
  statusRef.current = state.status;
  lastEventIdRef.current = state.lastEventId;

  // 切换会话或历史刷新时重水合（运行中不覆盖流状态）。
  useEffect(() => {
    void threadId;
    if (
      statusRef.current === "running" ||
      statusRef.current === "awaiting_human"
    ) {
      return;
    }
    const includeThinking = shouldShowThinking(capabilities);
    const toolCallDetail = toolCallDetailMode(capabilities);
    setState({
      ...createEmptyStreamState(),
      items: hydrateTimelineFromMessages(historyMessages ?? [], {
        includeThinking,
        toolCallDetail,
      }),
    });
    activeRunIdRef.current = null;
    reconnectAttemptRef.current = 0;
  }, [threadId, historyMessages, capabilities]);

  useEffect(() => {
    return () => {
      intentionalAbortRef.current = true;
      streamAbortRef.current?.();
    };
  }, []);

  // HITL 超时轮询
  useEffect(() => {
    if (state.status !== "awaiting_human" || !state.pendingHitlId) {
      return;
    }
    const requestId = state.pendingHitlId;
    const item = state.items.find(
      (i) => i.kind === "hitl" && i.id === requestId,
    );
    if (!item || item.kind !== "hitl" || !item.request.timeoutMs) {
      return;
    }
    const started = hitlStartedAtRef.current.get(requestId) ?? Date.now();
    hitlStartedAtRef.current.set(requestId, started);
    const remain = item.request.timeoutMs - (Date.now() - started);
    const timer = setTimeout(
      () => {
        setState((prev) => markHitlTimedOut(prev, requestId));
      },
      Math.max(0, remain),
    );
    return () => clearTimeout(timer);
  }, [state.status, state.pendingHitlId, state.items]);

  const reduceOptions = useCallback(() => {
    return {
      includeThinking: shouldShowThinking(capabilities),
      toolCallDetail: toolCallDetailMode(capabilities),
    };
  }, [capabilities]);

  const attachHandlers = useCallback(
    (runIdHint?: string) => {
      const api = getAgentApi();
      return {
        onEvent: (event: AguiEvent) => {
          if (event.type === "RUN_STARTED") {
            activeRunIdRef.current = event.runId;
          }
          if (event.type === "CUSTOM" && event.name === "hitl_request") {
            const value = event.value as { requestId?: string } | undefined;
            if (value?.requestId) {
              hitlStartedAtRef.current.set(value.requestId, Date.now());
            }
          }
          setState((prev) => reduceAguiEvent(prev, event, reduceOptions()));
        },
        onError: (error: Error) => {
          if (intentionalAbortRef.current) {
            return;
          }
          if (
            error instanceof AgentApiError &&
            error.code === "AGENT_STREAM_CURSOR_INVALID"
          ) {
            setState((prev) =>
              markStreamConnection(
                {
                  ...prev,
                  error: {
                    message: error.message,
                    code: error.code,
                    traceId: error.traceId ?? undefined,
                  },
                },
                "cursor_invalid",
              ),
            );
            return;
          }

          const runId = activeRunIdRef.current ?? runIdHint ?? null;
          const canReconnect =
            (statusRef.current === "running" ||
              statusRef.current === "awaiting_human") &&
            runId &&
            threadId &&
            reconnectAttemptRef.current < 3;

          if (canReconnect && threadId && runId) {
            reconnectAttemptRef.current += 1;
            setState((prev) => markStreamConnection(prev, "reconnecting"));
            const handle = api.reconnectRunStream(
              threadId,
              runId,
              {
                cursor: lastEventIdRef.current,
                lastEventId: lastEventIdRef.current,
              },
              attachHandlers(runId),
            );
            streamAbortRef.current = () => handle.abort();
            return;
          }

          setState((prev) => ({
            ...markStreamDisconnected(prev),
            error: {
              message: error.message,
              code:
                error instanceof AgentApiError ? error.code : "STREAM_ERROR",
              traceId:
                error instanceof AgentApiError
                  ? (error.traceId ?? undefined)
                  : undefined,
            },
          }));
        },
        onDone: () => {
          streamAbortRef.current = null;
          if (
            statusRef.current === "running" ||
            statusRef.current === "awaiting_human"
          ) {
            // 正常结束由 RUN_FINISHED 归约；若无终态事件则保持。
          }
        },
      };
    },
    [reduceOptions, threadId],
  );

  const send = useCallback(
    async (text: string, model?: string) => {
      if (!threadId) {
        return;
      }
      if (
        statusRef.current === "running" ||
        statusRef.current === "awaiting_human"
      ) {
        return;
      }
      intentionalAbortRef.current = false;
      reconnectAttemptRef.current = 0;
      const localId = `local_${Date.now()}`;
      setState((prev) => ({
        ...appendLocalUserMessage(prev, text, localId),
        status: "running",
        connection: "connecting",
        error: undefined,
      }));

      const body: CreateRunRequest = {
        input: { text },
        model,
        onDisconnect: "continue",
      };
      const api = getAgentApi();
      const handle = api.streamRun(threadId, body, attachHandlers());
      streamAbortRef.current = () => handle.abort();
      try {
        const runId = await handle.runId;
        activeRunIdRef.current = runId;
        setState((prev) => ({
          ...prev,
          runId,
          connection: "connected",
        }));
      } catch {
        // runId 解析失败时仍依赖 RUN_STARTED。
      }
    },
    [threadId, attachHandlers],
  );

  const steer = useCallback(
    async (text: string) => {
      if (!threadId || !text.trim()) {
        return;
      }
      if (capabilities?.steering !== true) {
        return;
      }
      if (
        statusRef.current !== "running" &&
        statusRef.current !== "awaiting_human"
      ) {
        return;
      }
      const api = getAgentApi();
      await api.steer(threadId, { input: { text } });
      const id = `steer_${Date.now()}`;
      setState((prev) => appendSteerMessage(prev, text, id));
    },
    [threadId, capabilities],
  );

  const respondHitl = useCallback(
    async (
      requestId: string,
      decision: HitlDecision,
      modifiedText?: string,
    ) => {
      if (!threadId) {
        return;
      }
      const item = state.items.find(
        (i) => i.kind === "hitl" && i.id === requestId,
      );
      if (!item || item.kind !== "hitl") {
        return;
      }
      const started = hitlStartedAtRef.current.get(requestId);
      const check = validateHitlResponse(
        item.request,
        decision,
        modifiedText,
        Date.now(),
        started,
      );
      if (!check.ok) {
        setState((prev) => ({
          ...prev,
          error: { message: check.reason, code: "AGENT_VALIDATION_FAILED" },
        }));
        return;
      }
      const api = getAgentApi();
      await api.respondHitl(threadId, {
        requestId,
        decision,
        modifiedInput:
          decision === "modify" ? { text: modifiedText } : undefined,
      });
      setState((prev) =>
        applyHitlDecision(prev, requestId, decision, modifiedText),
      );
    },
    [threadId, state.items],
  );

  const cancel = useCallback(async () => {
    if (!threadId) {
      return;
    }
    intentionalAbortRef.current = true;
    streamAbortRef.current?.();
    streamAbortRef.current = null;
    const runId = activeRunIdRef.current;
    if (!runId) {
      setState((prev) => applyLocalCancel(prev, "cancelled"));
      return;
    }
    try {
      const result = await getAgentApi().cancelRun(threadId, runId);
      setState((prev) => applyLocalCancel(prev, result.status));
    } catch (error) {
      setState((prev) => ({
        ...applyLocalCancel(prev, "cancelled"),
        error: {
          message: error instanceof Error ? error.message : String(error),
          code: error instanceof AgentApiError ? error.code : undefined,
          traceId:
            error instanceof AgentApiError
              ? (error.traceId ?? undefined)
              : undefined,
        },
      }));
    } finally {
      activeRunIdRef.current = null;
    }
  }, [threadId]);

  const reconnectHint = useCallback(() => {
    setState((prev) => ({
      ...createEmptyStreamState(),
      connection: "connected",
    }));
    activeRunIdRef.current = null;
    reconnectAttemptRef.current = 0;
  }, []);

  const manualReconnect = useCallback(() => {
    if (!threadId || !activeRunIdRef.current) {
      reconnectHint();
      return;
    }
    intentionalAbortRef.current = false;
    reconnectAttemptRef.current = 0;
    setState((prev) => markStreamConnection(prev, "reconnecting"));
    const handle = getAgentApi().reconnectRunStream(
      threadId,
      activeRunIdRef.current,
      {
        cursor: lastEventIdRef.current,
        lastEventId: lastEventIdRef.current,
      },
      attachHandlers(activeRunIdRef.current),
    );
    streamAbortRef.current = () => handle.abort();
  }, [threadId, attachHandlers, reconnectHint]);

  const onToggleTool = useCallback((toolCallId: string) => {
    setState((prev) => toggleToolCollapsed(prev, toolCallId));
  }, []);

  const onToggleThinking = useCallback((messageId: string) => {
    setState((prev) => toggleThinkingCollapsed(prev, messageId));
  }, []);

  return {
    state,
    send,
    steer,
    cancel,
    respondHitl,
    reconnectHint,
    manualReconnect,
    onToggleTool,
    onToggleThinking,
  };
}
