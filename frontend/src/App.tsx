import React, { useState } from 'react';
import { useDevices } from './hooks/useDevices';
import { DeviceList } from './components/DeviceList';
import { FileBrowser } from './components/FileBrowser';
import { TransferQueue } from './components/TransferQueue';
import { QRCodeDisplay } from './components/QRCodeDisplay';
import { GetWiFiAddress, GetWiFiQRCode } from '../wailsjs/go/main/App';
import type { model } from '../wailsjs/go/models';
type Device = model.Device;
import './styles/app.css';

interface QRState {
    address: string;
    qrCode: string;
}

function App() {
    const { devices, loading, progress } = useDevices();
    const [selectedDevice, setSelectedDevice] = useState<Device | null>(null);
    const [qrState, setQrState] = useState<QRState | null>(null);

    const handleSelectDevice = (device: Device) => {
        setSelectedDevice(device);
    };

    const handleShowQR = async (_device: Device) => {
        try {
            const [address, qrCode] = await Promise.all([
                GetWiFiAddress(),
                GetWiFiQRCode(),
            ]);
            setQrState({ address, qrCode });
        } catch (err) {
            console.error('Failed to get WiFi info:', err);
        }
    };

    const handleCloseQR = () => {
        setQrState(null);
    };

    return (
        <div className="app-root">
            <div className="app-main">
                <aside className="app-sidebar">
                    <DeviceList
                        devices={devices}
                        loading={loading}
                        selectedDevice={selectedDevice}
                        onSelect={handleSelectDevice}
                        onShowQR={handleShowQR}
                    />
                </aside>
                <main className="app-content">
                    <FileBrowser device={selectedDevice} />
                </main>
            </div>
            <footer className="app-footer">
                <TransferQueue progress={progress} />
            </footer>

            {qrState && (
                <QRCodeDisplay
                    address={qrState.address}
                    qrCode={qrState.qrCode}
                    onClose={handleCloseQR}
                />
            )}
        </div>
    );
}

export default App;
