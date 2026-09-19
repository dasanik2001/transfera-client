import axios from 'axios';

/**
 * C++ API server base URL.
 * - Dev: .env.local → http://127.0.0.1:8080 (next dev rewrites /api)
 * - Production: .env.production / CI → https://transfera-api.onrender.com
 */
export const PRODUCTION_API_URL = 'https://transfera-api.onrender.com';

export const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_BASE_URL?.replace(/\/$/, '') ||
  (process.env.NODE_ENV === 'production'
    ? PRODUCTION_API_URL
    : 'http://127.0.0.1:8080');

export function apiUrl(path: string): string {
  const normalized = path.startsWith('/') ? path : `/${path}`;
  return `${API_BASE_URL}${normalized}`;
}

export interface RoomParticipant {
  id: string;
  name: string;
  isCreator: boolean;
  joinedAt: number;
}

export interface RoomMessage {
  id: string;
  type: 'system' | 'chat' | 'file';
  senderId: string;
  senderName: string;
  text: string;
  fileId?: string;
  fileName?: string;
  fileSize?: number;
  timestamp: number;
}

export interface RoomFile {
  id: string;
  name: string;
  size: number;
  uploaderId: string;
  uploaderName: string;
  uploadedAt: number;
}

export interface RoomSyncResponse {
  roomId: number;
  maxParticipants: number;
  participants: RoomParticipant[];
  messages: RoomMessage[];
  files: RoomFile[];
}

export interface CreateRoomResponse {
  status: string;
  roomId: number;
  port: number;
  userId: string;
  maxParticipants: number;
  message?: string;
}

export interface JoinRoomResponse {
  status: string;
  roomId: number;
  port: number;
  userId: string;
  maxParticipants: number;
  message?: string;
}

export async function createRoom(
  creatorName: string,
  maxParticipants: number = 5,
  port?: number
): Promise<CreateRoomResponse> {
  const res = await axios.post<CreateRoomResponse>(apiUrl('/api/rooms'), {
    creatorName: creatorName.trim() || 'Host',
    maxParticipants,
    ...(port ? { port } : {}),
  });
  return res.data;
}

export async function joinRoom(
  port: number,
  userName: string
): Promise<JoinRoomResponse> {
  const res = await axios.post<JoinRoomResponse>(apiUrl(`/api/rooms/${port}/join`), {
    userName: userName.trim() || 'Guest',
  });
  return res.data;
}

export async function leaveRoom(port: number, userId: string): Promise<void> {
  await axios.post(apiUrl(`/api/rooms/${port}/leave`), { userId });
}

export async function syncRoom(
  port: number,
  userId: string,
  since: number = 0
): Promise<RoomSyncResponse> {
  const res = await axios.get<RoomSyncResponse>(apiUrl(`/api/rooms/${port}/sync`), {
    params: { since, userId },
    timeout: 5000,
  });
  return res.data;
}

export async function sendRoomMessage(
  port: number,
  userId: string,
  text: string
): Promise<void> {
  await axios.post(apiUrl(`/api/rooms/${port}/messages`), { text, userId });
}

export async function uploadRoomFiles(
  port: number,
  userId: string,
  files: File[],
  note?: string
): Promise<void> {
  const formData = new FormData();
  formData.append('userId', userId);
  if (note) formData.append('note', note);
  files.forEach((file) => {
    formData.append('files', file);
  });

  await axios.post(apiUrl(`/api/rooms/${port}/upload`), formData);
}

export function getRoomFileDownloadUrl(
  port: number,
  fileId: string,
  inline = false
): string {
  const base = apiUrl(`/api/rooms/${port}/files/${fileId}`);
  return inline ? `${base}?inline=1` : base;
}

export async function removeRoomParticipant(
  port: number,
  byUserId: string,
  targetUserId: string
): Promise<void> {
  await axios.post(apiUrl(`/api/rooms/${port}/remove`), { byUserId, targetUserId });
}
