import { RotateCcw } from "lucide-react";
import type { AudioRecord } from "../../lib/api/types";
import {
  audioSourceLabel,
  audioStatusLabel,
  formatAudioSize,
  formatAudioTime,
} from "./audio-format";

type AudioHistoryTableProps = {
  records: AudioRecord[];
  isLoading: boolean;
  replayingId: number | null;
  onReplay: (recordId: number) => void;
};

export function AudioHistoryTable({
  records,
  isLoading,
  replayingId,
  onReplay,
}: AudioHistoryTableProps) {
  if (isLoading) {
    return <div className="empty-state">正在加载音频记录...</div>;
  }
  if (records.length === 0) {
    return <div className="empty-state">暂无入站音频记录。</div>;
  }

  return (
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th>接收时间</th>
            <th>状态</th>
            <th>来源</th>
            <th>文件</th>
            <th>大小</th>
            <th>摘要</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          {records.map((record) => (
            <tr key={record.id}>
              <td className="mono">{formatAudioTime(record.createdAt)}</td>
              <td>
                <span className={`badge ${record.status}`}>
                  {audioStatusLabel(record.status)}
                </span>
              </td>
              <td>
                {audioSourceLabel(record.sourceKind)}
                {record.source ? (
                  <span className="muted block">{record.source}</span>
                ) : null}
              </td>
              <td>
                {record.originalFilename || "-"}
                <span className="muted block">{record.mimeType || "-"}</span>
              </td>
              <td>{formatAudioSize(record.sizeBytes)}</td>
              <td>
                {record.errorSummary ? (
                  <span className="form-error">{record.errorSummary}</span>
                ) : (
                  <span className="mono">{record.sha256.slice(0, 12)}</span>
                )}
              </td>
              <td>
                <button
                  aria-label="重放音频"
                  className="icon-button"
                  disabled={replayingId === record.id}
                  onClick={() => onReplay(record.id)}
                  title="重放"
                  type="button"
                >
                  <RotateCcw aria-hidden="true" size={16} />
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
