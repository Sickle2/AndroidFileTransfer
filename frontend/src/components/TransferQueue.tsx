import React from 'react';
import type { TransferProgress } from '../hooks/useDevices';

interface TransferQueueProps {
    progress: TransferProgress[];
}

export function TransferQueue({ progress }: TransferQueueProps) {
    if (progress.length === 0) {
        return (
            <div className="transfer-queue transfer-queue--empty">
                <span className="transfer-queue-idle">无传输任务</span>
            </div>
        );
    }

    return (
        <div className="transfer-queue">
            {progress.map((item) => {
                const hasError = item.error && item.error.length > 0;
                // BytesDone === -1 is the backend's "transfer completed" sentinel
                // (ADB exec-based transfers can't report exact byte progress).
                const isDone = item.bytesDone === -1;
                // TotalBytes === 0 while not done/errored means the transfer has
                // started but no size is known yet: show an indeterminate state
                // instead of a misleading, permanently-0% bar.
                const isIndeterminate = !isDone && !hasError && item.totalBytes === 0;
                const pct = isDone
                    ? 100
                    : item.totalBytes > 0
                        ? Math.round((item.bytesDone / item.totalBytes) * 100)
                        : 0;

                return (
                    <div
                        key={`${item.deviceId}-${item.fileName}`}
                        className={`transfer-item ${hasError ? 'transfer-item--error' : ''} ${isDone ? 'transfer-item--done' : ''}`}
                    >
                        <span className="transfer-filename" title={item.fileName}>
                            {item.fileName}
                        </span>
                        <span className="transfer-device">{item.deviceId}</span>
                        {hasError ? (
                            <span className="transfer-error">{item.error}</span>
                        ) : isDone ? (
                            <span className="transfer-done">完成</span>
                        ) : isIndeterminate ? (
                            <div className="transfer-progress-wrap">
                                <div className="transfer-progress-bar">
                                    <div className="transfer-progress-fill transfer-progress-fill--indeterminate" />
                                </div>
                                <span className="transfer-pct">传输中...</span>
                            </div>
                        ) : (
                            <div className="transfer-progress-wrap">
                                <div className="transfer-progress-bar">
                                    <div
                                        className="transfer-progress-fill"
                                        style={{ width: `${pct}%` }}
                                    />
                                </div>
                                <span className="transfer-pct">{pct}%</span>
                            </div>
                        )}
                    </div>
                );
            })}
        </div>
    );
}
