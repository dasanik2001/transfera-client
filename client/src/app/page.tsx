'use client';

import { useEffect, useState } from 'react';
import FileUpload from '@/components/FileUpload';
import FileDownload from '@/components/FileDownload';
import InviteCode from '@/components/InviteCode';
import RoomView from '@/components/RoomView';
import axios from 'axios';
import { apiUrl, API_BASE_URL } from '@/lib/api';
import { sitePath, githubPagesUrl } from '@/lib/site';
import {
  getResponseHeader,
  resolveDownloadFilename,
} from '@/lib/downloadFilename';
import { validateUploadFile } from '@/lib/uploadValidation';
import {
  DEFAULT_MAX_DOWNLOADS,
  MAX_MAX_DOWNLOADS,
  MIN_MAX_DOWNLOADS,
  clampMaxDownloads,
  parseMaxDownloadsInput,
} from '@/lib/uploadLimits';

export default function Home() {
  const [uploadedFile, setUploadedFile] = useState<File | null>(null);
  const [isUploading, setIsUploading] = useState(false);
  const [isDownloading, setIsDownloading] = useState(false);
  const [port, setPort] = useState<number | null>(null);
  const [maxDownloads, setMaxDownloads] = useState(DEFAULT_MAX_DOWNLOADS);
  const [inviteMaxDownloads, setInviteMaxDownloads] = useState(DEFAULT_MAX_DOWNLOADS);
  const [activeTab, setActiveTab] = useState<'upload' | 'download' | 'rooms'>('upload');
  const [apiOnline, setApiOnline] = useState<boolean | null>(null);

  useEffect(() => {
    let cancelled = false;
    const check = async () => {
      try {
        await axios.get(apiUrl('/api/health'), { timeout: 3000 });
        if (!cancelled) setApiOnline(true);
      } catch {
        if (!cancelled) setApiOnline(false);
      }
    };
    check();
    const id = setInterval(check, 10000);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, []);

  const handleFileUpload = async (file: File) => {
    const validationError = validateUploadFile(file);
    if (validationError) {
      alert(validationError);
      setUploadedFile(null);
      setPort(null);
      return;
    }

    setUploadedFile(file);
    setIsUploading(true);

    const allowedDownloads = clampMaxDownloads(maxDownloads);

    try {
      const formData = new FormData();
      formData.append('file', file);
      formData.append('maxDownloads', String(allowedDownloads));

      // Do not set Content-Type manually — axios/browser must add the boundary parameter.
      const response = await axios.post(apiUrl('/api/upload'), formData);

      const invitePort = response.data?.port;
      if (typeof invitePort !== 'number' || invitePort <= 0) {
        throw new Error('Invalid response from server');
      }
      const serverMax =
        typeof response.data?.maxDownloads === 'number'
          ? clampMaxDownloads(response.data.maxDownloads)
          : allowedDownloads;
      setPort(invitePort);
      setInviteMaxDownloads(serverMax);
    } catch (error) {
      console.error('Error uploading file:', error);
      const message =
        axios.isAxiosError(error) && error.response?.data
          ? typeof error.response.data === 'string'
            ? error.response.data
            : JSON.stringify(error.response.data)
          : `Cannot reach API at ${API_BASE_URL}. Start the server: cd server && ./scripts/run.sh`;
      alert(message);
    } finally {
      setIsUploading(false);
    }
  };

  const handleDownload = async (port: number) => {
    setIsDownloading(true);

    try {
      const response = await axios.get(apiUrl(`/api/download/${port}`), {
        responseType: 'blob',
        // Accept both 200 (full) and 206 (partial/range) responses
        validateStatus: (status) => status === 200 || status === 206,
      });

      const filename = resolveDownloadFilename(response.headers);
      const mimeType =
        getResponseHeader(response.headers, 'content-type') ||
        'application/octet-stream';

      const url = window.URL.createObjectURL(
        new Blob([response.data], { type: mimeType ?? undefined })
      );
      const link = document.createElement('a');
      link.href = url;
      link.setAttribute('download', filename);
      document.body.appendChild(link);
      link.click();
      link.remove();
      window.URL.revokeObjectURL(url);
    } catch (error) {
      console.error('Error downloading file:', error);
      if (axios.isAxiosError(error) && error.response?.status === 404) {
        alert('This invite code was already used or is no longer valid.');
      } else {
        alert('Failed to download file. Please check the invite code and try again.');
      }
      throw error;
    } finally {
      setIsDownloading(false);
    }
  };

  return (
    <div className="container mx-auto px-4 py-8 max-w-4xl">
      <header className="text-center mb-12">
        <h1 className="text-4xl font-bold text-blue-600 mb-2">Transfera</h1>
        <p className="text-xl text-gray-600">Secure P2P File Sharing</p>
        {apiOnline === false && (
          <p className="mt-4 text-sm text-red-600 bg-red-50 border border-red-200 rounded-md px-4 py-2 inline-block">
            API offline at {API_BASE_URL}. Run:{' '}
            <code className="font-mono">cd server && ./scripts/run.sh</code>
          </p>
        )}
        {apiOnline === true && (
          <p className="mt-4 text-sm text-green-700">API connected</p>
        )}
      </header>

      <div className="bg-white rounded-lg shadow-lg p-6">
        <div className="flex items-end justify-between gap-4 border-b mb-6">
          <div className="flex">
            <button
              className={`px-4 py-2 font-medium ${activeTab === 'upload'
                ? 'text-blue-600 border-b-2 border-blue-600'
                : 'text-gray-500 hover:text-gray-700'
                }`}
              onClick={() => setActiveTab('upload')}
            >
              Share a File
            </button>
            <button
              className={`px-4 py-2 font-medium ${activeTab === 'download'
                ? 'text-blue-600 border-b-2 border-blue-600'
                : 'text-gray-500 hover:text-gray-700'
                }`}
              onClick={() => setActiveTab('download')}
            >
              Receive a File
            </button>
            <button
              className={`px-4 py-2 font-medium flex items-center gap-1.5 ${activeTab === 'rooms'
                ? 'text-blue-600 border-b-2 border-blue-600'
                : 'text-gray-500 hover:text-gray-700'
                }`}
              onClick={() => setActiveTab('rooms')}
            >
              <span>Rooms</span>
              <span className="text-[10px] uppercase font-bold bg-blue-100 text-blue-700 px-1.5 py-0.5 rounded-full">
                Chat &amp; Share
              </span>
            </button>
          </div>

          {activeTab === 'upload' && (
           <div className="flex flex-col sm:flex-row sm:items-end pb-2 shrink-0 gap-1 sm:gap-2">
  <label
    htmlFor="maxDownloads"
    className="text-sm font-medium text-gray-700"
  >
    Max downloads:
  </label>

  <input
    type="number"
    id="maxDownloads"
    min={MIN_MAX_DOWNLOADS}
    max={MAX_MAX_DOWNLOADS}
    value={maxDownloads}
    onChange={(e) =>
      setMaxDownloads(parseMaxDownloadsInput(e.target.value))
    }
    disabled={isUploading}
    className="input-field w-full sm:w-16 h-8 text-center"
    title="Invite is removed after this many successful downloads"
  />
</div>
          )}
        </div>

        {activeTab === 'upload' ? (
          <div>
            <FileUpload onFileUpload={handleFileUpload} isUploading={isUploading} />

            {uploadedFile && !isUploading && (
              <div className="mt-4 p-3 bg-gray-50 rounded-md">
                <p className="text-sm text-gray-600">
                  Selected file: <span className="font-medium">{uploadedFile.name}</span> ({Math.round(uploadedFile.size / 1024)} KB)
                </p>
              </div>
            )}

            {isUploading && (
              <div className="mt-6 text-center">
                <div className="inline-block animate-spin rounded-full h-8 w-8 border-4 border-blue-500 border-t-transparent"></div>
                <p className="mt-2 text-gray-600">Uploading file...</p>
              </div>
            )}

            <InviteCode port={port} maxDownloads={inviteMaxDownloads} />
          </div>
        ) : activeTab === 'download' ? (
          <div>
            <FileDownload onDownload={handleDownload} isDownloading={isDownloading} />

            {isDownloading && (
              <div className="mt-6 text-center">
                <div className="inline-block animate-spin rounded-full h-8 w-8 border-4 border-blue-500 border-t-transparent"></div>
                <p className="mt-2 text-gray-600">Downloading file...</p>
              </div>
            )}
          </div>
        ) : (
          <RoomView />
        )}
      </div>

      <footer className="mt-12 text-center text-gray-500 text-sm space-y-1">
        <p>Transfera &copy; {new Date().getFullYear()} - Secure P2P File Sharing</p>
        <p>
          <a
            href={sitePath('/manual/index.html')}
            className="text-blue-600 hover:underline"
            target="_blank"
            rel="noopener noreferrer"
          >
            Technical manual (architecture &amp; internals)
          </a>
          {' · '}
          <a
            href={githubPagesUrl('/manual/')}
            className="text-blue-600 hover:underline"
            target="_blank"
            rel="noopener noreferrer"
          >
            Manual on GitHub Pages
          </a>
        </p>
      </footer>
    </div>
  );
}
