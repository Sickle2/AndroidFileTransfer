import { useState, useEffect, useRef } from 'react';
import { ListDevices } from '../../wailsjs/go/main/App';
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime';
import type { model } from '../../wailsjs/go/models';
type Device = model.Device;

export interface TransferProgress {
    deviceId: string;
    fileName: string;
    bytesDone: number;
    totalBytes: number;
    error?: string;
}

export function useDevices() {
    const [devices, setDevices] = useState<Device[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [progress, setProgress] = useState<TransferProgress[]>([]);
    const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

    const fetchDevices = async () => {
        try {
            const result = await ListDevices();
            setDevices(result || []);
            setError(null);
        } catch (err) {
            setError(err instanceof Error ? err.message : String(err));
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        fetchDevices();
        intervalRef.current = setInterval(fetchDevices, 2000);

        EventsOn('transfer:progress', (data: TransferProgress) => {
            setProgress(prev => {
                const idx = prev.findIndex(
                    p => p.deviceId === data.deviceId && p.fileName === data.fileName
                );
                if (idx >= 0) {
                    const updated = [...prev];
                    updated[idx] = data;
                    return updated;
                }
                return [...prev, data];
            });
        });

        return () => {
            if (intervalRef.current) {
                clearInterval(intervalRef.current);
            }
            EventsOff('transfer:progress');
        };
    }, []);

    return { devices, loading, error, progress };
}
