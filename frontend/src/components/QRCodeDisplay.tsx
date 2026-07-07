import React from 'react';

interface QRCodeDisplayProps {
    address: string;
    qrCode: string;
    onClose: () => void;
}

export function QRCodeDisplay({ address, qrCode, onClose }: QRCodeDisplayProps) {
    const handleCopy = () => {
        navigator.clipboard.writeText(address).catch(() => {
            // fallback: select text
        });
    };

    const handleBackdropClick = (e: React.MouseEvent<HTMLDivElement>) => {
        if (e.target === e.currentTarget) {
            onClose();
        }
    };

    return (
        <div className="modal-backdrop" onClick={handleBackdropClick} role="dialog" aria-modal="true" aria-label="WiFi QR Code">
            <div className="modal-content">
                <div className="modal-header">
                    <h3>Connect via WiFi</h3>
                    <button className="modal-close-btn" onClick={onClose} aria-label="Close">✕</button>
                </div>
                <div className="modal-body">
                    {qrCode && (
                        <img
                            src={qrCode}
                            alt="WiFi QR Code"
                            className="qr-image"
                        />
                    )}
                    {!qrCode && <div className="qr-placeholder">Loading QR code...</div>}
                    <div className="qr-address">
                        <span className="qr-address-label">Address:</span>
                        <code className="qr-address-value">{address}</code>
                    </div>
                </div>
                <div className="modal-footer">
                    <button className="btn btn-primary" onClick={handleCopy}>
                        Copy Link
                    </button>
                    <button className="btn btn-secondary" onClick={onClose}>
                        Close
                    </button>
                </div>
            </div>
        </div>
    );
}
