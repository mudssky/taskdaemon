import { Copy, RefreshCw, ShieldCheck, Volume2 } from "lucide-react";
import { useMemo, useState } from "react";
import { AudioHistoryTable } from "./AudioHistoryTable";
import {
  useAudioConfigQuery,
  useAudioHistoryQuery,
  useReplayAudioMutation,
} from "./audio.queries";
import { formatAudioSize } from "./audio-format";

const urlEndpoint = "/api/inbound/audio-play-requests";
const uploadEndpoint = "/api/inbound/audio-play-requests/upload";

export function AudioSettingsPage() {
  const configQuery = useAudioConfigQuery();
  const historyQuery = useAudioHistoryQuery();
  const replayMutation = useReplayAudioMutation();
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
  const audioConfig = configQuery.data;
  const historyLimit = audioConfig?.history.limit ?? 50;
  const maxBytes = audioConfig?.inbound.maxBytes ?? 209715200;
  const allowedSchemes = audioConfig?.inbound.url.allowedSchemes ?? ["https"];

  async function copyExample() {
    await navigator.clipboard.writeText(curlExample);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1500);
  }

  return (
    <div className="settings-grid">
      <section className="panel wide" aria-labelledby="audio-basic-title">
        <div className="panel-header">
          <div className="settings-list-item">
            <h2 id="audio-basic-title">音频播放</h2>
            <p>入站音频会保存本地副本，后端播放按队列顺序执行。</p>
          </div>
          <button
            className="button subtle"
            disabled={historyQuery.isFetching}
            onClick={() => historyQuery.refetch()}
            type="button"
          >
            <RefreshCw aria-hidden="true" size={16} />
            刷新
          </button>
        </div>
        <div className="settings-summary">
          <div className="settings-summary-item">
            <span className="metric-icon">
              <Volume2 aria-hidden="true" size={18} />
            </span>
            <div>
              <p className="eyebrow">播放目标</p>
              <strong>{audioConfig?.autoplay.target ?? "backend"}</strong>
              <span>
                {audioConfig?.autoplay.enabled
                  ? "自动播放已开启"
                  : "自动播放已关闭"}
              </span>
            </div>
          </div>
          <div className="settings-summary-item">
            <span className="metric-icon">
              <ShieldCheck aria-hidden="true" size={18} />
            </span>
            <div>
              <p className="eyebrow">入站认证</p>
              <strong>Bearer Token</strong>
              <span>
                {audioConfig?.inbound.tokenConfigured
                  ? "已配置 token hash"
                  : "未配置 token hash"}
              </span>
            </div>
          </div>
        </div>
      </section>

      <section className="panel" aria-labelledby="audio-inbound-title">
        <div className="panel-header">
          <div className="settings-list-item">
            <h2 id="audio-inbound-title">入站接口</h2>
            <p>Token 在配置文件中以 hash 保存，不会在页面回显。</p>
          </div>
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
          <div>
            <dt>默认上限</dt>
            <dd>{formatAudioSize(maxBytes)}</dd>
          </div>
          <div>
            <dt>默认协议</dt>
            <dd>{allowedSchemes.join(", ")}</dd>
          </div>
          <div className="settings-list-item">
            <dt>队列上限</dt>
            <dd>
              {audioConfig?.playback.queueLimit === 0
                ? "无上限"
                : (audioConfig?.playback.queueLimit ?? 20)}
            </dd>
          </div>
          <div className="settings-list-item">
            <dt>FFmpeg</dt>
            <dd>
              {audioConfig?.ffmpeg.pathConfigured
                ? "已配置路径"
                : "内置或 PATH"}
            </dd>
          </div>
        </dl>
        {configQuery.isError ? (
          <p className="form-error">音频配置状态加载失败。</p>
        ) : null}
      </section>

      <section className="panel" aria-labelledby="audio-example-title">
        <div className="panel-header">
          <div>
            <h2 id="audio-example-title">调用示例</h2>
            <p>把 token 替换为配置文件里对应 hash 的原始 token。</p>
          </div>
          <button className="icon-button" onClick={copyExample} type="button">
            <Copy aria-hidden="true" size={16} />
          </button>
        </div>
        <pre>{curlExample}</pre>
        {copied ? <p className="muted">已复制</p> : null}
      </section>

      <section className="panel wide" aria-labelledby="audio-history-title">
        <div className="panel-header">
          <div>
            <h2 id="audio-history-title">最近记录</h2>
            <p>
              最近 {historyLimit === 0 ? "全部" : historyLimit}{" "}
              条入站音频，可手动重放。
            </p>
          </div>
        </div>
        {historyQuery.isError ? (
          <div className="empty-state error">
            音频记录加载失败，请确认登录状态和后端服务。
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
    </div>
  );
}
