import React from 'react';
import type { Device } from '../../wailsjs/go/main/App';

interface DeviceListProps {
    devices: Device[];
    loading: boolean;
    selectedDevice: Device | null;
    onSelect: (device: Device) => void;
    onShowQR: (device: Device) => void;
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
        case 'connected': return 'Connected';
        case 'disconnected': return 'Disconnected';
        case 'available': return 'Available';
        default: return status;
    }
}

export function DeviceList({ devices, loading, selectedDevice, onSelect, onShowQR }: DeviceListProps) {
    const hasAdb = devices.some(d => d.type.toLowerCase() === 'adb');

    const handleDeviceClick = (device: Device) => {
        onSelect(device);
        if (device.type.toLowerCase() === 'wifi') {
            onShowQR(device);
        }
    };

    return (
        <div className="device-list">
            <div className="device-list-header">
                <h3>Devices</h3>
            </div>

            {loading && devices.length === 0 && (
                <div className="device-list-empty">Scanning for devices...</div>
            )}

            {!loading && devices.length === 0 && (
                <div className="device-list-empty">No devices found</div>
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
                        {device.type.toLowerCase() === 'wifi' && (
                            <button
                                className="device-qr-btn"
                                title="Show QR Code"
                                onClick={e => { e.stopPropagation(); onShowQR(device); }}
                            >
                                QR
                            </button>
                        )}
                    </li>
                ))}
            </ul>

            {!hasAdb && (
                <div className="adb-hint">
                    <p>ADB not detected. To connect via USB, install Android Debug Bridge (ADB) and enable USB debugging on your device.</p>
                </div>
            )}
        </div>
    );
}
