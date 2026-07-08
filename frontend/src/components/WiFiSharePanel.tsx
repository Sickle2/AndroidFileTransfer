import React, { useState, useEffect, useCallback } from 'react';
import {
    GetWiFiAddress,
    GetWiFiQRCode,
    GetShareConfig,
    SetShareMode,
    SetRootDir,
    AddSharedPaths,
    RemoveSharedItem,
    ClearSharedItems,
    SetUploadDir,
} from '../../wailsjs/go/main/App';
import type { model } from '../../wailsjs/go/models';

type ShareConfig = model.ShareConfig;

// runtime is the Wails runtime injected on window. Dialogs are optional at
// build time so we guard every call.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const runtime = () => (window as any).runtime;

async function pickFiles(): Promise<string[]> {
    const rt = runtime();
    if (rt && typeof rt.OpenMultipleFilesDialog === 'function') {
        const paths: string[] | null = await rt.OpenMultipleFilesDialog({ Title: '选择要共享的文件' });
        return paths || [];
    }
    if (rt && typeof rt.OpenFileDialog === 'function') {
        const path: string = await rt.OpenFileDialog({ Title: '选择要共享的文件' });
        return path ? [path] : [];
    }
    return [];
}

async function pickDirectory(title: string): Promise<string> {
    const rt = runtime();
    if (rt && typeof rt.OpenDirectoryDialog === 'function') {
        const path: string = await rt.OpenDirectoryDialog({ Title: title });
        return path || '';
    }
    return '';
}

export function WiFiSharePanel() {
    const [address, setAddress] = useState('');
    const [qrCode, setQrCode] = useState('');
    const [config, setConfig] = useState<ShareConfig | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [busy, setBusy] = useState(false);

    const refreshConfig = useCallback(async () => {
        try {
            const cfg = await GetShareConfig();
            setConfig(cfg);
        } catch (err) {
            setError(err instanceof Error ? err.message : String(err));
        }
    }, []);

    useEffect(() => {
        let cancelled = false;
        (async () => {
            try {
                const [addr, qr, cfg] = await Promise.all([
                    GetWiFiAddress(),
                    GetWiFiQRCode(),
                    GetShareConfig(),
                ]);
                if (!cancelled) {
                    setAddress(addr);
                    setQrCode(qr);
                    setConfig(cfg);
                }
            } catch (err) {
                if (!cancelled) {
                    setError(err instanceof Error ? err.message : String(err));
                }
            }
        })();
        return () => {
            cancelled = true;
        };
    }, []);

    // Wrap a backend mutation: run it, refresh config, surface errors.
    const run = useCallback(
        async (fn: () => Promise<void>) => {
            setBusy(true);
            setError(null);
            try {
                await fn();
                await refreshConfig();
            } catch (err) {
                setError(err instanceof Error ? err.message : String(err));
            } finally {
                setBusy(false);
            }
        },
        [refreshConfig],
    );

    const handleModeChange = (mode: 'selected' | 'directory') => {
        if (!config || config.mode === mode) return;
        if (mode === 'directory') {
            const ok = window.confirm(
                '高级模式会把整个文件夹（含所有子文件）暴露给连接的手机。\n' +
                    '请确认该文件夹内没有隐私文件。是否继续？',
            );
            if (!ok) return;
        }
        run(() => SetShareMode(mode));
    };

    const handleAddFiles = () =>
        run(async () => {
            const paths = await pickFiles();
            if (paths.length > 0) await AddSharedPaths(paths);
        });

    const handleAddFolder = () =>
        run(async () => {
            const dir = await pickDirectory('选择要共享的文件夹');
            if (dir) await AddSharedPaths([dir]);
        });

    const handleChooseRoot = () =>
        run(async () => {
            const dir = await pickDirectory('选择共享根目录');
            if (dir) await SetRootDir(dir);
        });

    const handleChooseUpload = () =>
        run(async () => {
            const dir = await pickDirectory('选择接收目录');
            if (dir) await SetUploadDir(dir);
        });

    const handleRemove = (id: string) => run(() => RemoveSharedItem(id));
    const handleClear = () => run(() => ClearSharedItems());

    const mode = config?.mode ?? 'selected';

    return (
        <div className="wifi-share-panel">
            <div className="wifi-share-qr">
                {qrCode ? (
                    <img src={qrCode} alt="WiFi 二维码" className="qr-image" />
                ) : (
                    <div className="qr-placeholder">二维码生成中...</div>
                )}
                <div className="qr-address">
                    <span className="qr-address-label">地址：</span>
                    <code className="qr-address-value">{address}</code>
                </div>
                <p className="wifi-panel-hint">用手机扫码访问，在手机浏览器中传输文件</p>
            </div>

            {error && <div className="file-browser-error">{error}</div>}

            <div className="share-section">
                <div className="share-mode-tabs" role="tablist" aria-label="共享模式">
                    <button
                        role="tab"
                        aria-selected={mode === 'selected'}
                        className={`share-mode-tab ${mode === 'selected' ? 'is-active' : ''}`}
                        onClick={() => handleModeChange('selected')}
                        disabled={busy}
                    >
                        仅共享选定项
                    </button>
                    <button
                        role="tab"
                        aria-selected={mode === 'directory'}
                        className={`share-mode-tab ${mode === 'directory' ? 'is-active' : ''}`}
                        onClick={() => handleModeChange('directory')}
                        disabled={busy}
                    >
                        共享整个文件夹（高级）
                    </button>
                </div>

                {mode === 'selected' ? (
                    <div className="share-selected">
                        <div className="share-actions">
                            <button className="btn btn-primary" onClick={handleAddFiles} disabled={busy}>
                                添加文件
                            </button>
                            <button className="btn btn-primary" onClick={handleAddFolder} disabled={busy}>
                                添加文件夹
                            </button>
                            {config && config.sharedItems.length > 0 && (
                                <button className="btn btn-secondary" onClick={handleClear} disabled={busy}>
                                    清空
                                </button>
                            )}
                        </div>

                        {config && config.sharedItems.length === 0 ? (
                            <p className="share-empty-hint">
                                还没有共享任何文件。点击上方按钮添加文件或文件夹，
                                手机端只能看到你选择的内容。
                            </p>
                        ) : (
                            <ul className="share-item-list">
                                {config?.sharedItems.map(item => (
                                    <li key={item.id} className="share-item">
                                        <span className="file-icon">{item.isDir ? '📁' : '📄'}</span>
                                        <span className="share-item-name">{item.name}</span>
                                        <button
                                            className="share-item-remove"
                                            onClick={() => handleRemove(item.id)}
                                            disabled={busy}
                                            title="移除"
                                            aria-label={`移除 ${item.name}`}
                                        >
                                            ✕
                                        </button>
                                    </li>
                                ))}
                            </ul>
                        )}
                    </div>
                ) : (
                    <div className="share-directory">
                        <p className="share-warning">
                            ⚠️ 高级模式下，整个根目录及其所有子文件都会暴露给连接的手机。
                            隐藏文件（以 . 开头）不会显示。
                        </p>
                        <div className="share-path-row">
                            <span className="share-path-label">共享根目录：</span>
                            <code className="share-path-value">{config?.rootDir || '未设置'}</code>
                            <button className="btn btn-secondary" onClick={handleChooseRoot} disabled={busy}>
                                选择文件夹
                            </button>
                        </div>
                    </div>
                )}
            </div>

            <div className="share-section">
                <div className="share-path-row">
                    <span className="share-path-label">接收目录：</span>
                    <code className="share-path-value">{config?.uploadDir || '未设置'}</code>
                    <button className="btn btn-secondary" onClick={handleChooseUpload} disabled={busy}>
                        选择接收目录
                    </button>
                </div>
                <p className="wifi-panel-hint">手机上传的文件统一保存到此目录。</p>
            </div>
        </div>
    );
}
