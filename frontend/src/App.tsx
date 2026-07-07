import React, { useState, useEffect } from 'react';
import { useDevices } from './hooks/useDevices';
import { DeviceList } from './components/DeviceList';
import { FileBrowser } from './components/FileBrowser';
import { TransferQueue } from './components/TransferQueue';
import { GetWiFiAddress, GetWiFiQRCode } from '../wailsjs/go/main/App';
import type { model } from '../wailsjs/go/models';
type Device = model.Device;
import './styles/app.css';

interface WiFiInfo {
    address: string;
    qrCode: string;
}

function App() {
    const { devices, loading, progress } = useDevices();
    const [selectedDevice, setSelectedDevice] = useState<Device | null>(null);
    const [wifiInfo, setWifiInfo] = useState<WiFiInfo | null>(null);

    const handleSelectDevice = (device: Device) => {
        setSelectedDevice(device);
    };

    // WiFi devices act only as a QR-code entry point on the desktop: the Mac
    // runs the HTTP server and the phone's browser connects to it directly.
    // Fetch the address/QR code whenever a WiFi device becomes selected so it
    // can be shown inline instead of the FileBrowser.
    useEffect(() => {
        if (!selectedDevice || selectedDevice.type.toLowerCase() !== 'wifi') {
            setWifiInfo(null);
            return;
        }
        let cancelled = false;
        (async () => {
            try {
                const [address, qrCode] = await Promise.all([
                    GetWiFiAddress(),
                    GetWiFiQRCode(),
                ]);
                if (!cancelled) {
                    setWifiInfo({ address, qrCode });
                }
            } catch (err) {
                console.error('Failed to get WiFi info:', err);
            }
        })();
        return () => {
            cancelled = true;
        };
    }, [selectedDevice]);

    const isWiFiSelected = selectedDevice?.type.toLowerCase() === 'wifi';

    return (
        <div className="app-root">
            <div className="app-main">
                <aside className="app-sidebar">
                    <DeviceList
                        devices={devices}
                        loading={loading}
                        selectedDevice={selectedDevice}
                        onSelect={handleSelectDevice}
                    />
                </aside>
                <main className="app-content">
                    {isWiFiSelected ? (
                        <div className="wifi-panel">
                            {wifiInfo?.qrCode ? (
                                <img src={wifiInfo.qrCode} alt="WiFi 二维码" className="qr-image" />
                            ) : (
                                <div className="qr-placeholder">二维码生成中...</div>
                            )}
                            <div className="qr-address">
                                <span className="qr-address-label">地址：</span>
                                <code className="qr-address-value">{wifiInfo?.address ?? ''}</code>
                            </div>
                            <p className="wifi-panel-hint">用手机扫码访问，在手机浏览器中传输文件</p>
                        </div>
                    ) : (
                        <FileBrowser device={selectedDevice} />
                    )}
                </main>
            </div>
            <footer className="app-footer">
                <TransferQueue progress={progress} />
            </footer>
        </div>
    );
}

export default App;
