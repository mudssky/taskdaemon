/**
 * 绑定一次 stream run 到 StreamViewState。
 */

import type {
  AgentMessage,
  CreateRunRequest,
  RuntimeCapabilities,
} from "@taskdaemon/agent-protocol";
import { useCallback, useEffect, useRef, useState } from "react";
import { getAgentApi } from "../../lib/api/agent-api";
import { AgentApiError } from "../../lib/api/errors";
import { shouldShowThinking, toolCallDetailMode } from "./capability-ui";
import {
  appendLocalUserMessage,
  applyLocalCancel,
  createEmptyStreamState,
  hydrateTimelineFromMessages,
  markStreamDisconnected,
  reduceAguiEvent,
  type StreamViewState,
  toggleToolCollapsed,
} from "./stream-reducer";

type UseRunStreamArgs = {
  threadId: string | undefined;
  capabilities: RuntimeCapabilities | undefined;
  historyMessages: AgentMessage[] | undefined;
};

/**
 * 会话流式发送 / 中断 / 时间线状态。
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
  const statusRef = useRef(state.status);
  statusRef.current = state.status;

  // 切换会话或历史刷新时重水合（运行中不覆盖流状态）。
  useEffect(() => {
    if (!threadId) {
      setState(createEmptyStreamState());
      return;
    }
    if (statusRef.current === "running") {
      return;
    }
    const items = hydrateTimelineFromMessages(historyMessages ?? [], {
      includeThinking: shouldShowThinking(capabilities),
      toolCallDetail: toolCallDetailMode(capabilities),
    });
    setState({
      ...createEmptyStreamState(),
      items,
    });
  }, [threadId, historyMessages, capabilities]);

  useEffect(() => {
    return () => {
      streamAbortRef.current?.();
    };
  }, []);

  const send = useCallback(
    async (text: string, model?: string) => {
      if (!threadId || !text.trim()) {
        return;
      }
      if (state.status === "running") {
        return;
      }

      const localId = `local_${Date.now()}`;
      setState((prev) =>
        appendLocalUserMessage(
          {
            ...prev,
            status: "running",
            connection: "connecting",
            error: undefined,
          },
          text.trim(),
          localId,
        ),
      );

      const body: CreateRunRequest = {
        input: { text: text.trim() },
        model,
        onDisconnect: "continue",
      };

      const options = {
        includeThinking: shouldShowThinking(capabilities),
        toolCallDetail: toolCallDetailMode(capabilities),
      };

      const handle = getAgentApi().streamRun(threadId, body, {
        onEvent: (event) => {
          if (event.type === "RUN_STARTED") {
            activeRunIdRef.current = event.runId;
          }
          setState((prev) => reduceAguiEvent(prev, event, options));
        },
        onError: (error) => {
          const traceId = error instanceof AgentApiError ? error.traceId : null;
          const code = error instanceof AgentApiError ? error.code : undefined;
          setState((prev) =>
            markStreamDisconnected({
              ...prev,
              status: "error",
              connection: "disconnected",
              error: {
                message: error.message,
                code,
                traceId: traceId ?? undefined,
              },
            }),
          );
          streamAbortRef.current = null;
          activeRunIdRef.current = null;
        },
        onDone: () => {
          streamAbortRef.current = null;
          activeRunIdRef.current = null;
          setState((prev) => ({
            ...prev,
            connection: "connected",
            status: prev.status === "running" ? "idle" : prev.status,
          }));
        },
      });

      streamAbortRef.current = handle.abort;
      try {
        activeRunIdRef.current = await handle.runId;
      } catch {
        // runId 解析失败时仍依赖 RUN_STARTED
      }
    },
    [threadId, capabilities, state.status],
  );

  const cancel = useCallback(async () => {
    if (!threadId) {
      return;
    }
    const runId = activeRunIdRef.current;
    streamAbortRef.current?.();
    if (runId) {
      try {
        const result = await getAgentApi().cancelRun(threadId, runId);
        setState((prev) => applyLocalCancel(prev, result.status));
      } catch (error) {
        setState((prev) =>
          applyLocalCancel(
            {
              ...prev,
              error: {
                message:
                  error instanceof Error ? error.message : "cancel failed",
                code: error instanceof AgentApiError ? error.code : undefined,
                traceId:
                  error instanceof AgentApiError
                    ? (error.traceId ?? undefined)
                    : undefined,
              },
            },
            "cancelled",
          ),
        );
      }
    } else {
      setState((prev) => applyLocalCancel(prev, "cancelled"));
    }
    activeRunIdRef.current = null;
    streamAbortRef.current = null;
  }, [threadId]);

  const reconnectHint = useCallback(() => {
    // G4：不做自动续传；仅提示后让用户重新拉历史。
    setState((prev) => ({
      ...prev,
      connection: "connected",
      error: undefined,
    }));
  }, []);

  const onToggleTool = useCallback((toolCallId: string) => {
    setState((prev) => toggleToolCollapsed(prev, toolCallId));
  }, []);

  return {
    state,
    send,
    cancel,
    reconnectHint,
    onToggleTool,
  };
}
