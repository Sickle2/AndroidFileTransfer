import React from 'react';
import type { model } from '../../wailsjs/go/models';
type Device = model.Device;

interface DeviceListProps {
    devices: Device[];
    loading: boolean;
    selectedDevice: Device | null;
    onSelect: (device: Device) => void;
}

function deviceIcon(type: string): string {
    switch (type.toLowerCase()) {
        case 'wifi': return '📶';
        case 'adb': return '🔌';
        case 'mtp': return '📱';
        default: return '📱';
    }
}

function statusLabel(status: string): string {
    switch (status.toLowerCase()) {
        case 'connected': return '已连接';
        case 'disconnected': return '已断开';
        case 'available': return '可用';
        default: return status;
    }
}

export function DeviceList({ devices, loading, selectedDevice, onSelect }: DeviceListProps) {
    const hasAdb = devices.some(d => d.type.toLowerCase() === 'adb');

    const handleDeviceClick = (device: Device) => {
        onSelect(device);
    };

    return (
        <div className="device-list">
            <div className="device-list-header">
                <h3>设备</h3>
            </div>

            {loading && devices.length === 0 && (
                <div className="device-list-empty">正在扫描设备...</div>
            )}

            {!loading && devices.length === 0 && (
                <div className="device-list-empty">未发现设备</div>
            )}

            <ul className="device-list-items">
                {devices.map(device => (
                    <li
                        key={device.id}
                        className={`device-item ${selectedDevice?.id === device.id ? 'device-item--selected' : ''} ${device.status.toLowerCase() !== 'connected' ? 'device-item--inactive' : ''}`}
                        onClick={() => handleDeviceClick(device)}
                        role="button"
                        tabIndex={0}
                        onKeyDown={e => e.key === 'Enter' && handleDeviceClick(device)}
                    >
                        <span className="device-icon">{deviceIcon(device.type)}</span>
                        <span className="device-info">
                            <span className="device-name">{device.name}</span>
                            <span className={`device-status device-status--${device.status.toLowerCase()}`}>
                                {statusLabel(device.status)}
                            </span>
                        </span>
                    </li>
                ))}
            </ul>

            {!hasAdb && (
                <div className="adb-hint">
                    <p>未检测到 ADB。如需通过 USB 连接，请安装 Android Debug Bridge (ADB) 并在设备上启用 USB 调试。</p>
                </div>
            )}
        </div>
    );
}
