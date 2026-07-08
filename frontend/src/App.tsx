import React, { useState } from 'react';
import { useDevices } from './hooks/useDevices';
import { DeviceList } from './components/DeviceList';
import { FileBrowser } from './components/FileBrowser';
import { WiFiSharePanel } from './components/WiFiSharePanel';
import { TransferQueue } from './components/TransferQueue';
import type { model } from '../wailsjs/go/models';
type Device = model.Device;
import './styles/app.css';

function App() {
    const { devices, loading, progress } = useDevices();
    const [selectedDevice, setSelectedDevice] = useState<Device | null>(null);

    const handleSelectDevice = (device: Device) => {
        setSelectedDevice(device);
    };

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
                        <WiFiSharePanel />
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
