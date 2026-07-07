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
            {progress.map((item, idx) => {
                const pct = item.totalBytes > 0
                    ? Math.round((item.bytesDone / item.totalBytes) * 100)
                    : 0;
                const hasError = item.error && item.error.length > 0;

                return (
                    <div
                        key={`${item.deviceId}-${item.fileName}`}
                        className={`transfer-item ${hasError ? 'transfer-item--error' : ''}`}
                    >
                        <span className="transfer-filename" title={item.fileName}>
                            {item.fileName}
                        </span>
                        <span className="transfer-device">{item.deviceId}</span>
                        {hasError ? (
                            <span className="transfer-error">{item.error}</span>
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
