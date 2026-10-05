'use client';

import { useState } from 'react';
import {
  FiCopy,
  FiCheck,
  FiFile,
  FiArchive,
  FiTrash2,
  FiChevronDown,
  FiChevronUp,
} from 'react-icons/fi';
import { formatFileSize } from '@/lib/uploadValidation';

export interface ActiveShare {
  id: string;
  port: number;
  filename: string;
  size: number;
  maxDownloads: number;
  isZip?: boolean;
  bundledFiles?: string[];
  createdAt: number;
}

interface InviteCodeProps {
  port?: number | null;
  maxDownloads?: number;
  shares?: ActiveShare[];
  onRemoveShare?: (id: string) => void;
  onClearAll?: () => void;
}

export default function InviteCode({
  port,
  maxDownloads = 1,
  shares = [],
  onRemoveShare,
  onClearAll,
}: InviteCodeProps) {
  const [copiedPort, setCopiedPort] = useState<number | null>(null);
  const [copiedAll, setCopiedAll] = useState(false);
  const [expandedZipId, setExpandedZipId] = useState<string | null>(null);

  // If modern shares array is provided, use it; otherwise fallback to single legacy port
  const items: ActiveShare[] =
    shares.length > 0
      ? shares
      : port
      ? [
          {
            id: 'legacy',
            port,
            filename: 'Shared File',
            size: 0,
            maxDownloads,
            createdAt: Date.now(),
          },
        ]
      : [];

  if (items.length === 0) return null;

  const copyCode = (sharePort: number) => {
    navigator.clipboard.writeText(sharePort.toString());
    setCopiedPort(sharePort);
    setTimeout(() => setCopiedPort(null), 2000);
  };

  const copyAllCodes = () => {
    const text = items
      .map((item) => `${item.filename}: ${item.port}`)
      .join('\n');
    navigator.clipboard.writeText(text);
    setCopiedAll(true);
    setTimeout(() => setCopiedAll(false), 2000);
  };

  return (
    <div className="mt-6 space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-lg font-semibold text-green-900 flex items-center gap-2">
            <span>Ready to Share!</span>
            <span className="text-xs bg-green-200 text-green-800 px-2 py-0.5 rounded-full font-medium">
              {items.length} {items.length === 1 ? 'Share' : 'Shares'}
            </span>
          </h3>
          <p className="text-sm text-green-700">
            Share the invite code{items.length > 1 ? 's' : ''} with the recipient to begin downloading:
          </p>
        </div>

        {items.length > 1 && (
          <div className="flex items-center gap-2">
            <button
              onClick={copyAllCodes}
              className="text-xs font-medium px-3 py-1.5 bg-green-100 hover:bg-green-200 text-green-800 rounded-md transition-colors flex items-center gap-1.5"
              title="Copy all codes and filenames to clipboard"
            >
              {copiedAll ? <FiCheck className="w-3.5 h-3.5" /> : <FiCopy className="w-3.5 h-3.5" />}
              <span>{copiedAll ? 'All Copied!' : 'Copy All Codes'}</span>
            </button>
            {onClearAll && (
              <button
                onClick={onClearAll}
                className="text-xs font-medium px-2.5 py-1.5 bg-gray-100 hover:bg-red-50 text-gray-600 hover:text-red-600 rounded-md transition-colors"
                title="Clear all active shares"
              >
                Clear
              </button>
            )}
          </div>
        )}
      </div>

      <div className="space-y-3">
        {items.map((share) => {
          const isCopied = copiedPort === share.port;
          const isExpanded = expandedZipId === share.id;

          return (
            <div
              key={share.id}
              className="p-4 bg-green-50 border border-green-200 rounded-lg transition-all hover:shadow-sm"
            >
              <div className="flex items-start justify-between gap-3 mb-2">
                <div className="flex items-center gap-2 min-w-0">
                  <div className="p-2 bg-green-100 text-green-700 rounded-md shrink-0">
                    {share.isZip ? (
                      <FiArchive className="w-5 h-5" />
                    ) : (
                      <FiFile className="w-5 h-5" />
                    )}
                  </div>
                  <div className="min-w-0">
                    <p className="font-medium text-gray-900 truncate" title={share.filename}>
                      {share.filename}
                    </p>
                    <p className="text-xs text-gray-500">
                      {share.size > 0 ? formatFileSize(share.size) : ''}
                      {share.isZip && share.bundledFiles && (
                        <span> · {share.bundledFiles.length} files bundled</span>
                      )}
                    </p>
                  </div>
                </div>

                <div className="flex items-center gap-2 shrink-0">
                  <span className="text-xs bg-white text-gray-600 border border-gray-200 px-2 py-0.5 rounded-full">
                    Max {share.maxDownloads} {share.maxDownloads === 1 ? 'dl' : 'dls'}
                  </span>
                  {onRemoveShare && (
                    <button
                      onClick={() => onRemoveShare(share.id)}
                      className="text-gray-400 hover:text-red-500 p-1 rounded transition-colors"
                      title="Dismiss this share"
                      aria-label="Dismiss share"
                    >
                      <FiTrash2 className="w-4 h-4" />
                    </button>
                  )}
                </div>
              </div>

              {/* Collapsible bundled files list if it is a ZIP */}
              {share.isZip && share.bundledFiles && share.bundledFiles.length > 0 && (
                <div className="mb-3 text-xs">
                  <button
                    onClick={() => setExpandedZipId(isExpanded ? null : share.id)}
                    className="text-blue-600 hover:underline flex items-center gap-1 font-medium"
                  >
                    <span>{isExpanded ? 'Hide bundled files' : 'View bundled files'}</span>
                    {isExpanded ? <FiChevronUp /> : <FiChevronDown />}
                  </button>
                  {isExpanded && (
                    <div className="mt-1.5 p-2 bg-white rounded border border-gray-200 max-h-32 overflow-y-auto space-y-1">
                      {share.bundledFiles.map((fn, idx) => (
                        <div key={idx} className="flex items-center gap-1.5 text-gray-600 truncate">
                          <span className="text-gray-400">•</span>
                          <span className="truncate">{fn}</span>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}

              {/* Invite Code Bar */}
              <div className="flex items-center">
                <div className="flex-1 bg-white p-3 rounded-l-md border border-r-0 border-gray-300 font-mono text-lg font-bold tracking-wider text-blue-700">
                  {share.port}
                </div>
                <button
                  onClick={() => copyCode(share.port)}
                  className="p-3 bg-blue-600 hover:bg-blue-700 text-white rounded-r-md transition-colors flex items-center justify-center min-w-[3.5rem]"
                  aria-label="Copy invite code"
                  title="Copy invite code"
                >
                  {isCopied ? <FiCheck className="w-5 h-5 text-white" /> : <FiCopy className="w-5 h-5" />}
                </button>
              </div>

              <p className="mt-2 text-xs text-gray-500">
                Allows up to {share.maxDownloads} download{share.maxDownloads === 1 ? '' : 's'} via P2P.
                Removed after downloads are exhausted.
              </p>
            </div>
          );
        })}
      </div>
    </div>
  );
}
