'use client';

import { useState, useCallback, useMemo } from 'react';
import { useDropzone, type FileRejection } from 'react-dropzone';
import {
  FiUpload,
  FiFile,
  FiX,
  FiArchive,
  FiLayers,
  FiAlertCircle,
} from 'react-icons/fi';
import {
  MAX_UPLOAD_BYTES,
  MAX_UPLOAD_MB,
  formatFileSize,
  calculateTotalSize,
} from '@/lib/uploadValidation';

import { isVideoFile } from '@/lib/videoCompression';

export type ShareMode = 'separate' | 'zip';

export interface UploadProgressInfo {
  step: 'compressing-video' | 'zipping' | 'uploading';
  current: number;
  total: number;
  filename: string;
  percent?: number;
}

interface FileUploadProps {
  onFileUpload: (
    files: File[],
    mode: ShareMode,
    archiveName?: string,
    compressVideosVP09?: boolean
  ) => Promise<void> | void;
  isUploading: boolean;
  uploadProgress?: UploadProgressInfo | null;
}

export default function FileUpload({
  onFileUpload,
  isUploading,
  uploadProgress,
}: FileUploadProps) {
  const [stagedFiles, setStagedFiles] = useState<File[]>([]);
  const [shareMode, setShareMode] = useState<ShareMode>('separate');
  const [archiveName, setArchiveName] = useState('transfera-bundle.zip');
  const [compressVideosVP09, setCompressVideosVP09] = useState(true);
  const [dragActive, setDragActive] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const totalBytes = useMemo(() => calculateTotalSize(stagedFiles), [stagedFiles]);
  const hasVideos = useMemo(() => stagedFiles.some((f) => isVideoFile(f)), [stagedFiles]);
  const isOverLimit = totalBytes > MAX_UPLOAD_BYTES;
  const usagePercent = Math.min(100, Math.round((totalBytes / MAX_UPLOAD_BYTES) * 100));

  const onDrop = useCallback(
    (acceptedFiles: File[]) => {
      if (acceptedFiles.length === 0) return;

      setStagedFiles((prev) => {
        // Filter out exact duplicate files already staged
        const existingKeys = new Set(prev.map((f) => `${f.name}-${f.size}-${f.lastModified}`));
        const newFiles = acceptedFiles.filter(
          (f) => !existingKeys.has(`${f.name}-${f.size}-${f.lastModified}`)
        );

        const updated = [...prev, ...newFiles];
        const newTotal = calculateTotalSize(updated);

        if (newTotal > MAX_UPLOAD_BYTES) {
          setError(
            `Total size exceeds the ${MAX_UPLOAD_MB} MB cap (${formatFileSize(
              newTotal
            )} selected across ${updated.length} files). Please remove some files to proceed.`
          );
        } else {
          setError(null);
        }

        return updated;
      });
    },
    []
  );

  const onDropRejected = useCallback((rejections: FileRejection[]) => {
    setDragActive(false);
    const firstErr = rejections[0]?.errors[0]?.message;
    setError(firstErr || 'One or more files could not be accepted.');
  }, []);

  const removeFile = (index: number) => {
    setStagedFiles((prev) => {
      const updated = prev.filter((_, i) => i !== index);
      const newTotal = calculateTotalSize(updated);
      if (newTotal <= MAX_UPLOAD_BYTES) {
        setError(null);
      }
      return updated;
    });
  };

  const clearAllFiles = () => {
    setStagedFiles([]);
    setError(null);
  };

  const handleStartShare = async () => {
    if (stagedFiles.length === 0 || isOverLimit || isUploading) return;
    await onFileUpload(stagedFiles, shareMode, archiveName, compressVideosVP09);
    // Note: page handles clearing staged files or keeping active shares
  };

  const { getRootProps, getInputProps } = useDropzone({
    onDrop,
    multiple: true,
    onDragEnter: () => setDragActive(true),
    onDragLeave: () => setDragActive(false),
    onDropAccepted: () => setDragActive(false),
    onDropRejected,
  });

  return (
    <div className="space-y-4">
      {/* Dropzone Area */}
      <div
        {...getRootProps()}
        className={`
          w-full p-8 border-2 border-dashed rounded-xl text-center cursor-pointer transition-all duration-200
          ${
            dragActive
              ? 'border-blue-500 bg-blue-50/80 scale-[0.99]'
              : 'border-gray-300 hover:border-blue-400 hover:bg-gray-50/80'
          }
          ${isUploading ? 'opacity-50 pointer-events-none' : ''}
        `}
      >
        <input {...getInputProps()} />
        <div className="flex flex-col items-center justify-center space-y-3">
          <div className="p-3 bg-blue-100 text-blue-600 rounded-full shadow-inner">
            <FiUpload className="w-6 h-6" />
          </div>
          <div>
            <p className="text-lg font-medium text-gray-800">
              Drag &amp; drop files here, or click to browse
            </p>
            <p className="text-sm text-gray-500 mt-1">
              Select multiple files · <span className="font-semibold text-gray-700">{MAX_UPLOAD_MB} MB total combined cap</span> · All file types supported
            </p>
          </div>
        </div>
      </div>

      {/* Error message */}
      {error && (
        <div className="p-3 bg-red-50 border border-red-200 rounded-lg flex items-start gap-2.5 text-red-700 text-sm">
          <FiAlertCircle className="w-5 h-5 shrink-0 text-red-500 mt-0.5" />
          <div>
            <p className="font-medium">File size limit exceeded</p>
            <p className="text-red-600">{error}</p>
          </div>
        </div>
      )}

      {/* Staged Files Section */}
      {stagedFiles.length > 0 && (
        <div className="bg-gray-50 border border-gray-200 rounded-xl p-4 space-y-4">
          {/* Header & Storage Quota Meter */}
          <div className="space-y-2">
            <div className="flex items-center justify-between text-sm">
              <span className="font-medium text-gray-700">
                Selected Files ({stagedFiles.length})
              </span>
              <div className="flex items-center gap-3">
                <span
                  className={`font-semibold ${
                    isOverLimit ? 'text-red-600' : 'text-gray-700'
                  }`}
                >
                  {formatFileSize(totalBytes)} / {MAX_UPLOAD_MB} MB
                </span>
                <button
                  type="button"
                  onClick={clearAllFiles}
                  disabled={isUploading}
                  className="text-xs text-gray-500 hover:text-red-600 transition-colors"
                >
                  Clear all
                </button>
              </div>
            </div>

            {/* Total progress bar */}
            <div className="w-full bg-gray-200 rounded-full h-2 overflow-hidden">
              <div
                className={`h-full transition-all duration-300 ${
                  isOverLimit
                    ? 'bg-red-500'
                    : usagePercent > 80
                    ? 'bg-amber-500'
                    : 'bg-blue-600'
                }`}
                style={{ width: `${Math.min(100, usagePercent)}%` }}
              />
            </div>
          </div>

          {/* List of Files */}
          <div className="max-h-48 overflow-y-auto space-y-2 pr-1">
            {stagedFiles.map((file, idx) => (
              <div
                key={`${file.name}-${idx}`}
                className="flex items-center justify-between p-2.5 bg-white rounded-lg border border-gray-200 text-sm hover:border-gray-300 transition-colors"
              >
                <div className="flex items-center gap-2.5 min-w-0">
                  <FiFile className="w-4 h-4 text-gray-400 shrink-0" />
                  <span className="truncate font-medium text-gray-800" title={file.name}>
                    {file.name}
                  </span>
                  <span className="text-xs text-gray-500 shrink-0">
                    ({formatFileSize(file.size)})
                  </span>
                </div>
                <button
                  type="button"
                  onClick={() => removeFile(idx)}
                  disabled={isUploading}
                  className="text-gray-400 hover:text-red-500 p-1 rounded transition-colors"
                  title="Remove this file"
                  aria-label="Remove file"
                >
                  <FiX className="w-4 h-4" />
                </button>
              </div>
            ))}
          </div>

          {/* Share Access Mode Toggle (shown when files are staged) */}
          <div className="pt-2 border-t border-gray-200 space-y-3">
            <label className="text-xs font-semibold uppercase tracking-wider text-gray-500 block">
              Share Access Format
            </label>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
              {/* Option 1: Separate Invite Codes */}
              <button
                type="button"
                onClick={() => setShareMode('separate')}
                disabled={isUploading}
                className={`p-3 rounded-lg border text-left transition-all flex items-start gap-3 ${
                  shareMode === 'separate'
                    ? 'border-blue-600 bg-blue-50/60 ring-2 ring-blue-500/20'
                    : 'border-gray-200 bg-white hover:border-gray-300'
                }`}
              >
                <div
                  className={`p-2 rounded-md shrink-0 ${
                    shareMode === 'separate'
                      ? 'bg-blue-600 text-white'
                      : 'bg-gray-100 text-gray-600'
                  }`}
                >
                  <FiLayers className="w-4 h-4" />
                </div>
                <div>
                  <p className="font-semibold text-gray-900 text-sm">
                    Separate Share Codes
                  </p>
                  <p className="text-xs text-gray-500 mt-0.5">
                    Generate an individual invite code for each file ({stagedFiles.length}{' '}
                    {stagedFiles.length === 1 ? 'code' : 'codes'}).
                  </p>
                </div>
              </button>

              {/* Option 2: Bundle as ZIP */}
              <button
                type="button"
                onClick={() => setShareMode('zip')}
                disabled={isUploading}
                className={`p-3 rounded-lg border text-left transition-all flex items-start gap-3 ${
                  shareMode === 'zip'
                    ? 'border-blue-600 bg-blue-50/60 ring-2 ring-blue-500/20'
                    : 'border-gray-200 bg-white hover:border-gray-300'
                }`}
              >
                <div
                  className={`p-2 rounded-md shrink-0 ${
                    shareMode === 'zip'
                      ? 'bg-blue-600 text-white'
                      : 'bg-gray-100 text-gray-600'
                  }`}
                >
                  <FiArchive className="w-4 h-4" />
                </div>
                <div>
                  <p className="font-semibold text-gray-900 text-sm">
                    Bundle into Single ZIP
                  </p>
                  <p className="text-xs text-gray-500 mt-0.5">
                    Package all {stagedFiles.length} files into one .zip archive (DEFLATE Level 9 + VP09 video compression).
                  </p>
                </div>
              </button>
            </div>

            {/* Custom ZIP filename & VP09 Video compression controls when in zip mode */}
            {shareMode === 'zip' && (
              <div className="space-y-2 pt-1">
                <div className="flex items-center gap-2 text-xs">
                  <label htmlFor="archiveName" className="font-medium text-gray-600 shrink-0">
                    Archive filename:
                  </label>
                  <input
                    id="archiveName"
                    type="text"
                    value={archiveName}
                    onChange={(e) => setArchiveName(e.target.value)}
                    disabled={isUploading}
                    placeholder="transfera-bundle.zip"
                    className="input-field py-1 px-2.5 text-xs flex-1 rounded border border-gray-300 bg-white font-mono"
                  />
                </div>

                {hasVideos && (
                  <div className="p-2.5 bg-blue-50/70 border border-blue-200 rounded-md">
                    <label className="flex items-center gap-2 text-xs text-gray-700 cursor-pointer">
                      <input
                        type="checkbox"
                        checked={compressVideosVP09}
                        onChange={(e) => setCompressVideosVP09(e.target.checked)}
                        disabled={isUploading}
                        className="rounded text-blue-600 focus:ring-blue-500 h-4 w-4"
                      />
                      <span className="font-medium text-blue-950">
                        Use VP09 video compression before zipping
                      </span>
                      <span className="text-[10px] bg-blue-600 text-white px-1.5 py-0.5 rounded font-bold uppercase">
                        VP09
                      </span>
                    </label>
                    <p className="text-[11px] text-blue-700 mt-1 pl-6">
                      Video files will be transcoded to VP09 WebM to maximize compression and reduce archive size.
                    </p>
                  </div>
                )}
              </div>
            )}
          </div>

          {/* Action Button & Status */}
          <div className="pt-2">
            <button
              type="button"
              onClick={handleStartShare}
              disabled={isUploading || isOverLimit || stagedFiles.length === 0}
              className={`
                w-full py-3 px-4 rounded-lg font-medium text-white shadow-sm transition-all flex items-center justify-center gap-2
                ${
                  isOverLimit
                    ? 'bg-red-400 cursor-not-allowed'
                    : isUploading
                    ? 'bg-blue-400 cursor-wait'
                    : 'bg-blue-600 hover:bg-blue-700 active:bg-blue-800'
                }
              `}
            >
              {isUploading ? (
                <>
                  <div className="inline-block animate-spin rounded-full h-4 w-4 border-2 border-white border-t-transparent" />
                  <span>
                    {uploadProgress?.step === 'compressing-video'
                      ? `Compressing video to VP09: ${uploadProgress.filename} (${uploadProgress.percent ?? 0}%)...`
                      : uploadProgress?.step === 'zipping'
                      ? `Compressing ZIP at Level 9 (${uploadProgress.percent ?? 0}%)...`
                      : uploadProgress?.step === 'uploading'
                      ? `Uploading file ${uploadProgress.current} of ${uploadProgress.total} (${uploadProgress.filename})...`
                      : 'Sharing files...'}
                  </span>
                </>
              ) : isOverLimit ? (
                <span>Total size exceeds {MAX_UPLOAD_MB} MB cap — remove files</span>
              ) : (
                <span>
                  Share {stagedFiles.length}{' '}
                  {stagedFiles.length === 1 ? 'File' : 'Files'} ({formatFileSize(totalBytes)})
                  {shareMode === 'zip' && stagedFiles.length > 1 ? ' as ZIP' : ''}
                </span>
              )}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
