/**
 * Video compression utilities using native browser MediaRecorder with VP09 (VP9) codec.
 */

const VIDEO_EXTENSIONS = new Set([
  'mp4',
  'mov',
  'avi',
  'mkv',
  'webm',
  'flv',
  'wmv',
  'm4v',
  '3gp',
  'ts',
  'ogv',
]);

export function isVideoFile(file: File): boolean {
  if (file.type && file.type.toLowerCase().startsWith('video/')) {
    return true;
  }
  const ext = file.name.split('.').pop()?.toLowerCase();
  return ext ? VIDEO_EXTENSIONS.has(ext) : false;
}

/**
 * Returns the best supported VP09 / VP9 WebM MIME type in the current browser, or null if unsupported.
 */
export function getSupportedVP09MimeType(): string | null {
  if (typeof window === 'undefined' || typeof MediaRecorder === 'undefined') {
    return null;
  }

  const candidateTypes = [
    'video/webm; codecs=vp09.00.10.08',
    'video/webm; codecs=vp09',
    'video/webm; codecs=vp9,opus',
    'video/webm; codecs=vp9',
    'video/webm',
  ];

  for (const type of candidateTypes) {
    if (MediaRecorder.isTypeSupported(type)) {
      return type;
    }
  }

  return null;
}

/**
 * Compresses/transcodes a video file to VP09 WebM client-side using MediaRecorder.
 * If transcoding fails, is unsupported, or does not reduce file size, returns the original file.
 */
export async function compressVideoToVP09(
  file: File,
  onProgress?: (percent: number) => void
): Promise<File> {
  const mimeType = getSupportedVP09MimeType();
  if (!mimeType || typeof window === 'undefined') {
    return file;
  }

  return new Promise<File>((resolve) => {
    let resolved = false;
    let url = '';

    const cleanupAndResolve = (resultFile: File) => {
      if (resolved) return;
      resolved = true;
      if (url) {
        try {
          URL.revokeObjectURL(url);
        } catch {}
      }
      resolve(resultFile);
    };

    // Safety timeout: max 60 seconds per video to avoid hanging the UI
    const timeoutId = setTimeout(() => {
      cleanupAndResolve(file);
    }, 60000);

    try {
      url = URL.createObjectURL(file);
      const video = document.createElement('video');
      video.muted = true;
      video.playsInline = true;
      video.preload = 'auto';
      video.src = url;

      video.onloadedmetadata = () => {
        try {
          const duration = video.duration;
          if (!duration || !Number.isFinite(duration) || duration <= 0) {
            clearTimeout(timeoutId);
            return cleanupAndResolve(file);
          }

          // Grab media stream from video element
          let stream: MediaStream | null = null;
          const vidAny = video as unknown as {
            captureStream?: () => MediaStream;
            mozCaptureStream?: () => MediaStream;
          };

          if (typeof vidAny.captureStream === 'function') {
            stream = vidAny.captureStream();
          } else if (typeof vidAny.mozCaptureStream === 'function') {
            stream = vidAny.mozCaptureStream();
          }

          if (!stream) {
            clearTimeout(timeoutId);
            return cleanupAndResolve(file);
          }

          // Target bitrates tailored for high-efficiency VP09
          const width = video.videoWidth || 1280;
          const height = video.videoHeight || 720;
          let videoBitsPerSecond = 1200000; // 1.2 Mbps baseline for 720p VP09
          if (width * height > 1920 * 1080) {
            videoBitsPerSecond = 2200000;
          } else if (width * height <= 640 * 480) {
            videoBitsPerSecond = 600000;
          }

          const recorder = new MediaRecorder(stream, {
            mimeType,
            videoBitsPerSecond,
          });

          const chunks: Blob[] = [];
          recorder.ondataavailable = (e) => {
            if (e.data && e.data.size > 0) {
              chunks.push(e.data);
            }
          };

          recorder.onstop = () => {
            clearTimeout(timeoutId);
            if (chunks.length === 0) {
              return cleanupAndResolve(file);
            }

            const blob = new Blob(chunks, { type: 'video/webm' });
            const dotIdx = file.name.lastIndexOf('.');
            const baseName = dotIdx !== -1 ? file.name.substring(0, dotIdx) : file.name;
            const vp09Name = `${baseName}.vp09.webm`;

            // If compressed result saved size, use the VP09 file
            if (blob.size < file.size) {
              const compressedFile = new File([blob], vp09Name, {
                type: 'video/webm',
                lastModified: Date.now(),
              });
              cleanupAndResolve(compressedFile);
            } else {
              // Original file was already smaller or more compressed
              cleanupAndResolve(file);
            }
          };

          recorder.onerror = () => {
            clearTimeout(timeoutId);
            cleanupAndResolve(file);
          };

          video.ontimeupdate = () => {
            if (duration > 0 && onProgress) {
              const pct = Math.min(99, Math.round((video.currentTime / duration) * 100));
              onProgress(pct);
            }
          };

          video.onended = () => {
            if (recorder.state === 'recording') {
              recorder.stop();
            }
          };

          // Accelerate playback to speed up transcoding
          try {
            video.playbackRate = 4.0;
          } catch {}

          recorder.start(100);
          video.play().catch(() => {
            clearTimeout(timeoutId);
            cleanupAndResolve(file);
          });
        } catch {
          clearTimeout(timeoutId);
          cleanupAndResolve(file);
        }
      };

      video.onerror = () => {
        clearTimeout(timeoutId);
        cleanupAndResolve(file);
      };
    } catch {
      clearTimeout(timeoutId);
      cleanupAndResolve(file);
    }
  });
}
