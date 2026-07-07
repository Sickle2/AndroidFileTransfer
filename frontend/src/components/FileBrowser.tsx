import React, { useState, useEffect } from 'react';
import { GetFileList, Download, Upload } from '../../wailsjs/go/main/App';
import type { Device, FileInfo } from '../../wailsjs/go/main/App';

interface FileBrowserProps {
    device: Device | null;
}

function formatSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}

function formatDate(isoString: string): string {
    try {
        return new Date(isoString).toLocaleDateString();
    } catch {
        return isoString;
    }
}

export function FileBrowser({ device }: FileBrowserProps) {
    const [currentPath, setCurrentPath] = useState('/');
    const [files, setFiles] = useState<FileInfo[]>([]);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        if (!device) {
            setFiles([]);
            setCurrentPath('/');
            return;
        }
        loadFiles(currentPath);
    }, [device, currentPath]);

    const loadFiles = async (path: string) => {
        if (!device) return;
        setLoading(true);
        setError(null);
        try {
            const result = await GetFileList(device.id, path);
            setFiles(result || []);
        } catch (err) {
            setError(err instanceof Error ? err.message : String(err));
            setFiles([]);
        } finally {
            setLoading(false);
        }
    };

    const handleDirectoryClick = (file: FileInfo) => {
        setCurrentPath(file.path);
    };

    const handleFileDownload = async (file: FileInfo) => {
        if (!device) return;
        try {
            // Use Wails runtime dialog to pick save location
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
            const runtime = (window as any).runtime;
            let localPath: string = '';
            if (runtime && typeof runtime.SaveFileDialog === 'function') {
                localPath = await runtime.SaveFileDialog({ DefaultFilename: file.name });
            } else {
                localPath = file.name;
            }
            if (!localPath) return;
            await Download(device.id, file.path, localPath);
        } catch (err) {
            console.error('Download failed:', err);
        }
    };

    const handleUpload = async () => {
        if (!device) return;
        try {
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
            const runtime = (window as any).runtime;
            let localPath: string = '';
            if (runtime && typeof runtime.OpenFileDialog === 'function') {
                localPath = await runtime.OpenFileDialog({});
            }
            if (!localPath) return;
            const fileName = localPath.split('/').pop() || localPath;
            const remotePath = currentPath.endsWith('/')
                ? `${currentPath}${fileName}`
                : `${currentPath}/${fileName}`;
            await Upload(device.id, localPath, remotePath);
            loadFiles(currentPath);
        } catch (err) {
            console.error('Upload failed:', err);
        }
    };

    const breadcrumbs = (() => {
        const parts = currentPath.split('/').filter(Boolean);
        const crumbs: { label: string; path: string }[] = [{ label: '/', path: '/' }];
        let acc = '';
        for (const part of parts) {
            acc += `/${part}`;
            crumbs.push({ label: part, path: acc });
        }
        return crumbs;
    })();

    const navigateUp = () => {
        if (currentPath === '/') return;
        const parts = currentPath.split('/').filter(Boolean);
        parts.pop();
        setCurrentPath(parts.length === 0 ? '/' : `/${parts.join('/')}`);
    };

    if (!device) {
        return (
            <div className="file-browser file-browser--empty">
                <p>请先选择设备</p>
            </div>
        );
    }

    return (
        <div className="file-browser">
            <div className="file-browser-toolbar">
                <div className="breadcrumbs" aria-label="Current path">
                    {breadcrumbs.map((crumb, i) => (
                        <React.Fragment key={crumb.path}>
                            {i > 0 && <span className="breadcrumb-sep">/</span>}
                            <button
                                className="breadcrumb-item"
                                onClick={() => setCurrentPath(crumb.path)}
                            >
                                {crumb.label}
                            </button>
                        </React.Fragment>
                    ))}
                </div>
                <div className="file-browser-actions">
                    <button
                        className="btn btn-secondary"
                        onClick={navigateUp}
                        disabled={currentPath === '/'}
                        title="返回上级"
                    >
                        ↑ 上级
                    </button>
                    <button
                        className="btn btn-primary"
                        onClick={handleUpload}
                        title="上传文件"
                    >
                        上传文件
                    </button>
                </div>
            </div>

            {loading && <div className="file-browser-status">加载中...</div>}
            {error && <div className="file-browser-error">{error}</div>}

            {!loading && !error && files.length === 0 && (
                <div className="file-browser-status">此目录为空</div>
            )}

            <div className="file-list" role="list">
                {files.map(file => (
                    <div
                        key={file.path}
                        className={`file-item ${file.isDir ? 'file-item--dir' : 'file-item--file'}`}
                        role="listitem"
                        onDoubleClick={() => file.isDir && handleDirectoryClick(file)}
                        onClick={() => !file.isDir && handleFileDownload(file)}
                        title={file.isDir ? '双击打开' : '点击下载'}
                    >
                        <span className="file-icon">{file.isDir ? '📁' : '📄'}</span>
                        <span className="file-name">{file.name}</span>
                        <span className="file-size">{file.isDir ? '' : formatSize(file.size)}</span>
                        <span className="file-date">{formatDate(file.modTime)}</span>
                    </div>
                ))}
            </div>
        </div>
    );
}
