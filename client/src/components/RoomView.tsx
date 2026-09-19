'use client';

import { useState, useEffect, useRef, useCallback } from 'react';
import {
  FiUsers,
  FiSend,
  FiPaperclip,
  FiDownload,
  FiCopy,
  FiCheck,
  FiLogOut,
  FiFile,
  FiX,
  FiPlusCircle,
  FiLogIn,
} from 'react-icons/fi';
import {
  createRoom,
  joinRoom,
  leaveRoom,
  syncRoom,
  sendRoomMessage,
  uploadRoomFiles,
  getRoomFileDownloadUrl,
  RoomMessage,
  RoomParticipant,
  RoomFile,
} from '@/lib/api';

function formatBytes(bytes: number): string {
  if (!bytes || bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
}

function formatTime(timestampMs: number): string {
  if (!timestampMs) return '';
  const d = new Date(timestampMs);
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

function isImageFile(fileName?: string): boolean {
  if (!fileName) return false;
  return /\.(png|jpe?g|gif|webp|svg)$/i.test(fileName);
}

export default function RoomView() {
  // Lobby state
  const [inRoom, setInRoom] = useState(false);
  const [roomId, setRoomId] = useState<number | null>(null);
  const [userId, setUserId] = useState<string>('');

  // Create form
  const [createName, setCreateName] = useState('');
  const [createCapacity, setCreateCapacity] = useState(5);
  const [createCustomPort, setCreateCustomPort] = useState('');
  const [isCreating, setIsCreating] = useState(false);

  // Join form
  const [joinPort, setJoinPort] = useState('');
  const [joinName, setJoinName] = useState('');
  const [isJoining, setIsJoining] = useState(false);

  // Error feedback in lobby
  const [lobbyError, setLobbyError] = useState('');

  // Active room state
  const [participants, setParticipants] = useState<RoomParticipant[]>([]);
  const [maxCapacity, setMaxCapacity] = useState(5);
  const [messages, setMessages] = useState<RoomMessage[]>([]);
  const [, setFiles] = useState<RoomFile[]>([]);
  const [inputText, setInputText] = useState('');
  const [selectedFiles, setSelectedFiles] = useState<File[]>([]);
  const [isSending, setIsSending] = useState(false);
  const [copied, setCopied] = useState(false);
  const [showMemberList, setShowMemberList] = useState(false);

  // Drag & drop state inside room
  const [isDraggingOverChat, setIsDraggingOverChat] = useState(false);

  const chatEndRef = useRef<HTMLDivElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Scroll chat to bottom
  const scrollToBottom = useCallback(() => {
    chatEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, []);

  useEffect(() => {
    if (inRoom) {
      scrollToBottom();
    }
  }, [messages, inRoom, scrollToBottom]);

  // Sync polling loop when in a room
  useEffect(() => {
    if (!inRoom || !roomId || !userId) return;

    let cancelled = false;
    const fetchSync = async () => {
      try {
        const data = await syncRoom(roomId, userId);
        if (cancelled) return;
        setParticipants(data.participants || []);
        setMaxCapacity(data.maxParticipants || 5);
        setMessages(data.messages || []);
        setFiles(data.files || []);
      } catch (err) {
        console.error('Room sync failed:', err);
      }
    };

    fetchSync();
    const intervalId = setInterval(fetchSync, 1500);

    return () => {
      cancelled = true;
      clearInterval(intervalId);
    };
  }, [inRoom, roomId, userId]);

  // Create Room Handler
  const handleCreateRoom = async (e: React.FormEvent) => {
    e.preventDefault();
    setLobbyError('');
    setIsCreating(true);

    try {
      const portNum = createCustomPort.trim() ? parseInt(createCustomPort.trim(), 10) : undefined;
      if (portNum !== undefined && (isNaN(portNum) || portNum < 1 || portNum > 65535)) {
        setLobbyError('Room ID / Port must be between 1 and 65535');
        setIsCreating(false);
        return;
      }

      const res = await createRoom(createName, createCapacity, portNum);
      setRoomId(res.roomId);
      setUserId(res.userId);
      setMaxCapacity(res.maxParticipants);
      setInRoom(true);
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { message?: string } } })?.response?.data?.message || 'Failed to create room';
      setLobbyError(msg);
    } finally {
      setIsCreating(false);
    }
  };

  // Join Room Handler
  const handleJoinRoom = async (e: React.FormEvent) => {
    e.preventDefault();
    setLobbyError('');
    setIsJoining(true);

    const portNum = parseInt(joinPort.trim(), 10);
    if (isNaN(portNum) || portNum < 1 || portNum > 65535) {
      setLobbyError('Please enter a valid Room ID / Port (1-65535)');
      setIsJoining(false);
      return;
    }

    try {
      const res = await joinRoom(portNum, joinName);
      setRoomId(res.roomId);
      setUserId(res.userId);
      setMaxCapacity(res.maxParticipants);
      setInRoom(true);
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { message?: string } } })?.response?.data?.message || 'Failed to join room';
      setLobbyError(msg);
    } finally {
      setIsJoining(false);
    }
  };

  // Leave Room Handler
  const handleLeaveRoom = async () => {
    if (roomId && userId) {
      try {
        await leaveRoom(roomId, userId);
      } catch (err) {
        console.error('Leave room error:', err);
      }
    }
    setInRoom(false);
    setRoomId(null);
    setUserId('');
    setMessages([]);
    setParticipants([]);
    setSelectedFiles([]);
    setInputText('');
  };

  // Copy Room ID to clipboard
  const handleCopyRoomId = () => {
    if (roomId) {
      navigator.clipboard.writeText(String(roomId));
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  // File selection handlers
  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files.length > 0) {
      const filesArray = Array.from(e.target.files);
      setSelectedFiles((prev) => [...prev, ...filesArray]);
    }
    if (fileInputRef.current) {
      fileInputRef.current.value = '';
    }
  };

  const removeSelectedFile = (index: number) => {
    setSelectedFiles((prev) => prev.filter((_, i) => i !== index));
  };

  // Drag & drop into chat window
  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDraggingOverChat(true);
  };

  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDraggingOverChat(false);
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDraggingOverChat(false);
    if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
      const filesArray = Array.from(e.dataTransfer.files);
      setSelectedFiles((prev) => [...prev, ...filesArray]);
    }
  };

  // Send message or files
  const handleSendMessage = async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    if (!inRoom || !roomId || !userId) return;

    const trimmedText = inputText.trim();
    if (!trimmedText && selectedFiles.length === 0) return;

    setIsSending(true);
    try {
      if (selectedFiles.length > 0) {
        await uploadRoomFiles(roomId, userId, selectedFiles, trimmedText || undefined);
        setSelectedFiles([]);
        setInputText('');
      } else if (trimmedText) {
        await sendRoomMessage(roomId, userId, trimmedText);
        setInputText('');
      }
      // Trigger instant sync
      const updated = await syncRoom(roomId, userId);
      setMessages(updated.messages || []);
      setFiles(updated.files || []);
      setParticipants(updated.participants || []);
    } catch (err) {
      console.error('Send message failed:', err);
      alert('Failed to send. Please check your connection.');
    } finally {
      setIsSending(false);
    }
  };

  // -------------------------------------------------------------
  // LOBBY VIEW
  // -------------------------------------------------------------
  if (!inRoom) {
    return (
      <div className="space-y-6">
        <div className="bg-gradient-to-r from-blue-50 to-indigo-50 p-5 rounded-xl border border-blue-100">
          <h2 className="text-xl font-bold text-gray-800 mb-1">Room-Based File Sharing &amp; Chat</h2>
          <p className="text-sm text-gray-600">
            Create a collaborative room with a custom participant capacity, share multiple files,
            and chat in real time with everyone in the room. Room ID is the port number!
          </p>
        </div>

        {lobbyError && (
          <div className="p-3 bg-red-50 border border-red-200 text-red-700 text-sm rounded-lg">
            {lobbyError}
          </div>
        )}

        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {/* Create Room Card */}
          <div className="bg-white border rounded-xl p-5 shadow-sm flex flex-col justify-between hover:border-blue-300 transition-colors">
            <div>
              <div className="flex items-center gap-2 mb-4 text-blue-600">
                <FiPlusCircle className="w-5 h-5" />
                <h3 className="text-lg font-semibold text-gray-900">Create a Room</h3>
              </div>

              <form onSubmit={handleCreateRoom} className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    Your Name
                  </label>
                  <input
                    type="text"
                    value={createName}
                    onChange={(e) => setCreateName(e.target.value)}
                    placeholder="e.g. Alice (Host)"
                    className="w-full px-3 py-2 border rounded-lg text-sm focus:ring-2 focus:ring-blue-500 focus:outline-none"
                    maxLength={30}
                    required
                  />
                </div>

                <div>
                  <div className="flex justify-between items-center mb-1">
                    <label className="text-sm font-medium text-gray-700">
                      Capacity (Max Participants)
                    </label>
                    <span className="text-xs font-semibold text-blue-600 bg-blue-50 px-2 py-0.5 rounded-full">
                      {createCapacity} people
                    </span>
                  </div>
                  <input
                    type="range"
                    min={2}
                    max={50}
                    value={createCapacity}
                    onChange={(e) => setCreateCapacity(parseInt(e.target.value, 10))}
                    className="w-full h-2 bg-gray-200 rounded-lg appearance-none cursor-pointer accent-blue-600"
                  />
                  <div className="flex justify-between text-[11px] text-gray-400 mt-1">
                    <span>2 (P2P pair)</span>
                    <span>10</span>
                    <span>25</span>
                    <span>50 max</span>
                  </div>
                </div>

                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    Custom Room ID / Port <span className="text-gray-400 font-normal">(Optional)</span>
                  </label>
                  <input
                    type="number"
                    value={createCustomPort}
                    onChange={(e) => setCreateCustomPort(e.target.value)}
                    placeholder="Auto-assigned if left blank"
                    min={1}
                    max={65535}
                    className="w-full px-3 py-2 border rounded-lg text-sm focus:ring-2 focus:ring-blue-500 focus:outline-none"
                  />
                  <p className="text-xs text-gray-400 mt-1">
                    Leave blank for an auto-generated port (49152-65535).
                  </p>
                </div>

                <button
                  type="submit"
                  disabled={isCreating}
                  className="w-full mt-2 py-2.5 px-4 bg-blue-600 hover:bg-blue-700 text-white font-medium text-sm rounded-lg shadow transition-all flex items-center justify-center gap-2 disabled:opacity-50"
                >
                  {isCreating ? (
                    <span>Creating Room...</span>
                  ) : (
                    <>
                      <FiPlusCircle className="w-4 h-4" />
                      <span>Create &amp; Enter Room</span>
                    </>
                  )}
                </button>
              </form>
            </div>
          </div>

          {/* Join Room Card */}
          <div className="bg-white border rounded-xl p-5 shadow-sm flex flex-col justify-between hover:border-blue-300 transition-colors">
            <div>
              <div className="flex items-center gap-2 mb-4 text-indigo-600">
                <FiLogIn className="w-5 h-5" />
                <h3 className="text-lg font-semibold text-gray-900">Join Existing Room</h3>
              </div>

              <form onSubmit={handleJoinRoom} className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    Room ID / Port
                  </label>
                  <input
                    type="number"
                    value={joinPort}
                    onChange={(e) => setJoinPort(e.target.value)}
                    placeholder="e.g. 52341"
                    min={1}
                    max={65535}
                    className="w-full px-3 py-2 border rounded-lg text-sm focus:ring-2 focus:ring-indigo-500 focus:outline-none font-mono"
                    required
                  />
                  <p className="text-xs text-gray-400 mt-1">
                    Ask the room creator for their 5-digit port number.
                  </p>
                </div>

                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    Your Display Name
                  </label>
                  <input
                    type="text"
                    value={joinName}
                    onChange={(e) => setJoinName(e.target.value)}
                    placeholder="e.g. Bob"
                    className="w-full px-3 py-2 border rounded-lg text-sm focus:ring-2 focus:ring-indigo-500 focus:outline-none"
                    maxLength={30}
                    required
                  />
                </div>

                <button
                  type="submit"
                  disabled={isJoining}
                  className="w-full mt-6 py-2.5 px-4 bg-indigo-600 hover:bg-indigo-700 text-white font-medium text-sm rounded-lg shadow transition-all flex items-center justify-center gap-2 disabled:opacity-50"
                >
                  {isJoining ? (
                    <span>Joining Room...</span>
                  ) : (
                    <>
                      <FiLogIn className="w-4 h-4" />
                      <span>Join Room</span>
                    </>
                  )}
                </button>
              </form>
            </div>
          </div>
        </div>
      </div>
    );
  }

  // -------------------------------------------------------------
  // ACTIVE ROOM VIEW (Chat UI & Multi-file sharing)
  // -------------------------------------------------------------
  return (
    <div
      className="flex flex-col h-[650px] bg-gray-50 border rounded-xl overflow-hidden shadow-md relative"
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
    >
      {/* Drag Over Overlay */}
      {isDraggingOverChat && (
        <div className="absolute inset-0 z-50 bg-blue-500/10 backdrop-blur-sm border-2 border-dashed border-blue-500 flex flex-col items-center justify-center pointer-events-none">
          <div className="bg-white p-4 rounded-xl shadow-lg flex items-center gap-3">
            <FiFile className="w-8 h-8 text-blue-500 animate-bounce" />
            <p className="text-sm font-medium text-gray-800">Drop files to share in room</p>
          </div>
        </div>
      )}

      {/* Room Header */}
      <header className="bg-white border-b px-4 py-3 flex items-center justify-between shrink-0">
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2">
            <span className="text-xs uppercase tracking-wider font-semibold text-gray-400">Room</span>
            <span className="font-mono text-lg font-bold text-gray-900 bg-gray-100 px-2.5 py-0.5 rounded border">
              {roomId}
            </span>
            <button
              onClick={handleCopyRoomId}
              title="Copy Room ID / Port"
              className="p-1.5 text-gray-500 hover:text-blue-600 hover:bg-gray-100 rounded transition-colors"
            >
              {copied ? <FiCheck className="w-4 h-4 text-green-600" /> : <FiCopy className="w-4 h-4" />}
            </button>
          </div>

          <div className="hidden sm:flex items-center gap-1.5 text-xs text-green-600 bg-green-50 px-2.5 py-1 rounded-full border border-green-200">
            <span className="w-2 h-2 rounded-full bg-green-500 animate-pulse"></span>
            <span>Live Sync</span>
          </div>
        </div>

        <div className="flex items-center gap-3">
          {/* Members indicator */}
          <div className="relative">
            <button
              onClick={() => setShowMemberList(!showMemberList)}
              className="flex items-center gap-1.5 text-xs font-medium text-gray-700 bg-gray-100 hover:bg-gray-200 px-3 py-1.5 rounded-lg transition-colors"
            >
              <FiUsers className="w-3.5 h-3.5 text-gray-500" />
              <span>
                {participants.length} / {maxCapacity}
              </span>
            </button>

            {/* Members popover */}
            {showMemberList && (
              <div className="absolute right-0 mt-2 w-56 bg-white border rounded-xl shadow-xl p-3 z-30 space-y-2">
                <div className="flex justify-between items-center pb-1 border-b">
                  <span className="text-xs font-semibold text-gray-500 uppercase tracking-wider">
                    Room Members
                  </span>
                  <button
                    onClick={() => setShowMemberList(false)}
                    className="text-gray-400 hover:text-gray-600"
                  >
                    <FiX className="w-3.5 h-3.5" />
                  </button>
                </div>
                <ul className="space-y-1.5 max-h-48 overflow-y-auto">
                  {participants.map((p) => (
                    <li key={p.id} className="flex items-center justify-between text-xs text-gray-700">
                      <span className="truncate">
                        {p.name} {p.id === userId && <span className="text-blue-600 font-semibold">(You)</span>}
                      </span>
                      {p.isCreator && (
                        <span className="text-[10px] bg-blue-100 text-blue-700 px-1.5 py-0.2 rounded font-medium">
                          Host
                        </span>
                      )}
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </div>

          {/* Leave Button */}
          <button
            onClick={handleLeaveRoom}
            className="flex items-center gap-1 text-xs font-medium text-red-600 hover:text-red-700 hover:bg-red-50 px-2.5 py-1.5 rounded-lg border border-red-200 transition-colors"
          >
            <FiLogOut className="w-3.5 h-3.5" />
            <span className="hidden sm:inline">Leave</span>
          </button>
        </div>
      </header>

      {/* Messages Feed */}
      <div className="flex-1 p-4 overflow-y-auto space-y-3">
        {messages.length === 0 && (
          <div className="h-full flex flex-col items-center justify-center text-gray-400 text-sm space-y-2">
            <FiUsers className="w-8 h-8 text-gray-300" />
            <p>Welcome to Room {roomId}! Start typing or drag &amp; drop files to share.</p>
          </div>
        )}

        {messages.map((msg) => {
          if (msg.type === 'system') {
            return (
              <div key={msg.id} className="flex justify-center my-2">
                <span className="text-[11px] font-medium bg-gray-200/70 text-gray-600 px-3 py-0.5 rounded-full">
                  {msg.text}
                </span>
              </div>
            );
          }

          const isMe = msg.senderId === userId;

          if (msg.type === 'file') {
            const isImage = isImageFile(msg.fileName);
            const downloadUrl = msg.fileId && roomId ? getRoomFileDownloadUrl(roomId, msg.fileId) : '#';
            const previewUrl = msg.fileId && roomId ? getRoomFileDownloadUrl(roomId, msg.fileId, true) : '';

            return (
              <div key={msg.id} className={`flex flex-col ${isMe ? 'items-end' : 'items-start'}`}>
                <div className="flex items-center gap-2 mb-1 px-1">
                  <span className="text-xs font-semibold text-gray-700">
                    {isMe ? 'You' : msg.senderName}
                  </span>
                  <span className="text-[10px] text-gray-400">{formatTime(msg.timestamp)}</span>
                </div>

                <div
                  className={`max-w-sm rounded-xl p-3 border shadow-sm ${
                    isMe
                      ? 'bg-blue-50 border-blue-200 text-gray-800'
                      : 'bg-white border-gray-200 text-gray-800'
                  }`}
                >
                  {msg.text && <p className="text-xs mb-2">{msg.text}</p>}

                  {/* Image Preview if applicable */}
                  {isImage && previewUrl && (
                    <div className="mb-2 rounded-lg overflow-hidden border bg-gray-100 max-h-48 flex items-center justify-center">
                      {/* eslint-disable-next-line @next/next/no-img-element */}
                      <img
                        src={previewUrl}
                        alt={msg.fileName || 'Shared file'}
                        className="object-contain max-h-48 w-full"
                        loading="lazy"
                      />
                    </div>
                  )}

                  {/* File card */}
                  <div className="flex items-center justify-between gap-3 bg-white/80 p-2.5 rounded-lg border border-gray-200">
                    <div className="flex items-center gap-2 min-w-0">
                      <div className="p-2 bg-blue-100 text-blue-600 rounded-lg shrink-0">
                        <FiFile className="w-5 h-5" />
                      </div>
                      <div className="min-w-0">
                        <p className="text-xs font-semibold text-gray-900 truncate" title={msg.fileName}>
                          {msg.fileName}
                        </p>
                        <p className="text-[10px] text-gray-500">{formatBytes(msg.fileSize || 0)}</p>
                      </div>
                    </div>

                    <a
                      href={downloadUrl}
                      download={msg.fileName}
                      className="p-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors shrink-0 flex items-center justify-center shadow-sm"
                      title="Download file"
                    >
                      <FiDownload className="w-4 h-4" />
                    </a>
                  </div>
                </div>
              </div>
            );
          }

          // Regular chat message
          return (
            <div key={msg.id} className={`flex flex-col ${isMe ? 'items-end' : 'items-start'}`}>
              <div className="flex items-center gap-2 mb-1 px-1">
                <span className="text-xs font-semibold text-gray-700">
                  {isMe ? 'You' : msg.senderName}
                </span>
                <span className="text-[10px] text-gray-400">{formatTime(msg.timestamp)}</span>
              </div>
              <div
                className={`max-w-md px-3.5 py-2 rounded-2xl text-sm leading-relaxed ${
                  isMe
                    ? 'bg-blue-600 text-white rounded-tr-none'
                    : 'bg-white border text-gray-800 rounded-tl-none shadow-sm'
                }`}
              >
                {msg.text}
              </div>
            </div>
          );
        })}
        <div ref={chatEndRef} />
      </div>

      {/* Selected Files Preview Bar */}
      {selectedFiles.length > 0 && (
        <div className="bg-white border-t px-4 py-2 flex flex-wrap gap-2 items-center">
          <span className="text-xs text-gray-500 font-medium">Ready to send:</span>
          {selectedFiles.map((file, idx) => (
            <span
              key={idx}
              className="flex items-center gap-1.5 bg-blue-50 border border-blue-200 text-blue-800 text-xs px-2.5 py-1 rounded-full"
            >
              <FiFile className="w-3.5 h-3.5 text-blue-500" />
              <span className="max-w-[120px] truncate">{file.name}</span>
              <span className="text-[10px] text-blue-500">({formatBytes(file.size)})</span>
              <button
                type="button"
                onClick={() => removeSelectedFile(idx)}
                className="text-blue-500 hover:text-red-500 ml-1"
              >
                <FiX className="w-3 h-3" />
              </button>
            </span>
          ))}
        </div>
      )}

      {/* Message Input Footer */}
      <footer className="bg-white border-t p-3 shrink-0">
        <form onSubmit={handleSendMessage} className="flex items-center gap-2">
          <input
            type="file"
            ref={fileInputRef}
            onChange={handleFileSelect}
            multiple
            className="hidden"
          />
          <button
            type="button"
            onClick={() => fileInputRef.current?.click()}
            title="Attach files (supports multiple)"
            className="p-2.5 text-gray-500 hover:text-blue-600 hover:bg-gray-100 rounded-lg transition-colors shrink-0"
          >
            <FiPaperclip className="w-5 h-5" />
          </button>

          <input
            type="text"
            value={inputText}
            onChange={(e) => setInputText(e.target.value)}
            placeholder={
              selectedFiles.length > 0
                ? 'Add an optional note with files...'
                : 'Type a message or drag & drop files here...'
            }
            className="flex-1 px-4 py-2 border rounded-lg text-sm focus:ring-2 focus:ring-blue-500 focus:outline-none"
            disabled={isSending}
          />

          <button
            type="submit"
            disabled={isSending || (!inputText.trim() && selectedFiles.length === 0)}
            className="p-2.5 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors shadow disabled:opacity-50 shrink-0"
            title="Send"
          >
            <FiSend className="w-4 h-4" />
          </button>
        </form>
      </footer>
    </div>
  );
}
