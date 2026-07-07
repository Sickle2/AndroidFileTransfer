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
        <div className="modal-backdrop" onClick={handleBackdropClick} role="dialog" aria-modal="true" aria-label="WiFi 二维码">
            <div className="modal-content">
                <div className="modal-header">
                    <h3>通过 WiFi 连接</h3>
                    <button className="modal-close-btn" onClick={onClose} aria-label="关闭">✕</button>
                </div>
                <div className="modal-body">
                    {qrCode && (
                        <img
                            src={qrCode}
                            alt="WiFi 二维码"
                            className="qr-image"
                        />
                    )}
                    {!qrCode && <div className="qr-placeholder">二维码生成中...</div>}
                    <div className="qr-address">
                        <span className="qr-address-label">地址：</span>
                        <code className="qr-address-value">{address}</code>
                    </div>
                </div>
                <div className="modal-footer">
                    <button className="btn btn-primary" onClick={handleCopy}>
                        复制链接
                    </button>
                    <button className="btn btn-secondary" onClick={onClose}>
                        关闭
                    </button>
                </div>
            </div>
        </div>
    );
}
