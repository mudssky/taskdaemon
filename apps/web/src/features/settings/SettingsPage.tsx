import { Copy, RefreshCw } from "lucide-react";
import { useMemo, useState } from "react";
import { ApiClientError } from "../../lib/api/client";
import {
  DesktopEnvironmentPanel,
  DesktopNativeFeaturesPanel,
  DesktopNotificationPanel,
} from "../../lib/desktop";
import { AudioHistoryTable } from "../audio/AudioHistoryTable";
import {
  useAudioHistoryQuery,
  useReplayAudioMutation,
} from "../audio/audio.queries";
import { AudioConfigForm } from "./AudioConfigForm";
import { useAudioSectionConfigQuery } from "./settings.queries";

const urlEndpoint = "/api/inbound/audio-play-requests";
const uploadEndpoint = "/api/inbound/audio-play-requests/upload";

/**
 * 完整设置页：音频可写配置 + 历史 + Desktop 能力面板。
 *
 * 返回值:
 *   - JSX.Element
 */
export function SettingsPage() {
  const configQuery = useAudioSectionConfigQuery();
  const historyQuery = useAudioHistoryQuery(
    configQuery.data?.history.limit ?? 50,
  );
  const replayMutation = useReplayAudioMutation();
  const [restartPaths, setRestartPaths] = useState<string[]>([]);
  const [copied, setCopied] = useState(false);

  const curlExample = useMemo(
    () =>
      [
        "curl -X POST http://127.0.0.1:39245/api/inbound/audio-play-requests",
        '  -H "Authorization: Bearer <token>"',
        '  -H "Content-Type: application/json"',
        '  -d \'{"url":"https://example.com/notice.mp3","source":"hermes"}\'',
      ].join(" \\\n"),
    [],
  );

  const loadTraceId =
    configQuery.error instanceof ApiClientError
      ? configQuery.error.traceId
      : null;

  async function copyExample() {
    await navigator.clipboard.writeText(curlExample);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1500);
  }

  function handleRestartRequired(paths: string[]) {
    setRestartPaths((prev) => {
      const next = new Set([...prev, ...paths]);
      return Array.from(next);
    });
  }

  return (
    <div className="settings-page">
      {restartPaths.length > 0 ? (
        <div className="restart-required-banner" role="status">
          <strong>配置已写入，需重启 taskdaemon 后生效</strong>
          <p>
            下列字段已落盘，但当前进程仍使用旧值：
            <span className="mono"> {restartPaths.join(", ")}</span>
          </p>
          <p className="muted">
            重启前相关行为不会切换。保存成功不代表运行态已热更新。
          </p>
        </div>
      ) : null}

      <div className="settings-grid">
        <AudioConfigForm
          config={configQuery.data}
          isLoading={configQuery.isLoading}
          isError={configQuery.isError}
          errorTraceId={loadTraceId}
          onRestartRequired={handleRestartRequired}
          onRefetch={() => {
            void configQuery.refetch();
          }}
        />

        <section className="panel" aria-labelledby="audio-example-title">
          <div className="panel-header">
            <div>
              <h2 id="audio-example-title">调用示例</h2>
              <p>把 token 替换为入站配置中的原始 Token（非 hash）。</p>
            </div>
            <button
              className="icon-button"
              onClick={() => {
                void copyExample();
              }}
              type="button"
              aria-label="复制 curl 示例"
            >
              <Copy aria-hidden="true" size={16} />
            </button>
          </div>
          <dl className="settings-list">
            <div className="settings-list-item">
              <dt>URL 播放</dt>
              <dd className="mono">{urlEndpoint}</dd>
            </div>
            <div className="settings-list-item">
              <dt>文件上传</dt>
              <dd className="mono">{uploadEndpoint}</dd>
            </div>
          </dl>
          <pre>{curlExample}</pre>
          {copied ? <p className="muted">已复制</p> : null}
        </section>

        <section className="panel wide" aria-labelledby="audio-history-title">
          <div className="panel-header">
            <div>
              <h2 id="audio-history-title">最近记录</h2>
              <p>入站音频历史，可手动重放。</p>
            </div>
            <button
              className="button subtle"
              disabled={historyQuery.isFetching}
              onClick={() => {
                void historyQuery.refetch();
              }}
              type="button"
            >
              <RefreshCw aria-hidden="true" size={16} />
              刷新
            </button>
          </div>
          {historyQuery.isError ? (
            <div className="empty-state error">
              音频记录加载失败，请确认登录状态和后端服务。
              {historyQuery.error instanceof ApiClientError &&
              historyQuery.error.traceId ? (
                <p className="muted mono">
                  traceId: {historyQuery.error.traceId}
                </p>
              ) : null}
            </div>
          ) : (
            <AudioHistoryTable
              isLoading={historyQuery.isLoading}
              onReplay={(recordId) => replayMutation.mutate(recordId)}
              records={historyQuery.data ?? []}
              replayingId={replayMutation.variables ?? null}
            />
          )}
          {replayMutation.isError ? (
            <p className="form-error">重放请求失败，请稍后刷新记录。</p>
          ) : null}
        </section>

        <DesktopEnvironmentPanel />
        <DesktopNotificationPanel />
        <DesktopNativeFeaturesPanel />
      </div>
    </div>
  );
}
