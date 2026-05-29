import type { AudioRecord, AudioRecordStatus } from "../../lib/api/types";
import { dayjs } from "../../lib/dayjs";

export function audioStatusLabel(status: AudioRecordStatus): string {
  switch (status) {
    case "received":
      return "已接收";
    case "queued":
      return "排队中";
    case "playing":
      return "播放中";
    case "played":
      return "已播放";
    case "failed":
      return "失败";
    case "skipped":
      return "已跳过";
    case "queued_skipped":
      return "队列已满";
  }
}

export function audioSourceLabel(
  sourceKind: AudioRecord["sourceKind"],
): string {
  switch (sourceKind) {
    case "url":
      return "URL";
    case "upload":
      return "上传";
  }
}

export function formatAudioTime(value: string): string {
  return dayjs(value).format("YYYY-MM-DD HH:mm:ss");
}

export function formatAudioSize(bytes: number): string {
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  const kib = bytes / 1024;
  if (kib < 1024) {
    return `${Math.round(kib * 10) / 10} KiB`;
  }
  const mib = kib / 1024;
  return `${Math.round(mib * 10) / 10} MiB`;
}
