import JSZip from 'jszip';
import { compressVideoToVP09, isVideoFile } from './videoCompression';

export interface ZipOptions {
  archiveName?: string;
  compressVideosVP09?: boolean;
  onProgress?: (percent: number) => void;
  onVideoProgress?: (filename: string, percent: number) => void;
}

export async function createZipBundle(
  files: File[],
  options?: ZipOptions | string,
  legacyOnProgress?: (percent: number) => void
): Promise<File> {
  const opts: ZipOptions =
    typeof options === 'string'
      ? { archiveName: options, onProgress: legacyOnProgress }
      : options || {};

  const archiveName = opts.archiveName || 'transfera-bundle.zip';
  const compressVideos = opts.compressVideosVP09 !== false;
  const onProgress = opts.onProgress || legacyOnProgress;
  const onVideoProgress = opts.onVideoProgress;

  // Process files: if video compression is enabled, compress videos with VP09 first
  const finalFiles: File[] = [];
  for (const file of files) {
    if (compressVideos && isVideoFile(file)) {
      onVideoProgress?.(file.name, 0);
      const compressedVideo = await compressVideoToVP09(file, (percent) => {
        onVideoProgress?.(file.name, percent);
      });
      finalFiles.push(compressedVideo);
    } else {
      finalFiles.push(file);
    }
  }

  const zip = new (JSZip as unknown as { new (): JSZip })();

  // Track filenames to avoid duplicates overwriting each other
  const nameCounts = new Map<string, number>();

  for (const file of finalFiles) {
    let filename = file.name;
    if (nameCounts.has(filename)) {
      const count = nameCounts.get(filename)! + 1;
      nameCounts.set(filename, count);
      const dotIndex = filename.lastIndexOf('.');
      if (dotIndex !== -1) {
        filename = `${filename.slice(0, dotIndex)} (${count})${filename.slice(dotIndex)}`;
      } else {
        filename = `${filename} (${count})`;
      }
    } else {
      nameCounts.set(filename, 0);
    }

    zip.file(filename, file);
  }

  // Best compression: DEFLATE Level 9 (maximum compression ratio)
  const blob = await zip.generateAsync(
    {
      type: 'blob',
      compression: 'DEFLATE',
      compressionOptions: {
        level: 9, // Maximum DEFLATE compression level
      },
    },
    (metadata: { percent: number }) => {
      if (onProgress) {
        onProgress(Math.round(metadata.percent));
      }
    }
  );

  const clean = archiveName.trim() || 'transfera-bundle.zip';
  const finalName = clean.endsWith('.zip') ? clean : `${clean}.zip`;

  return new File([blob], finalName, { type: 'application/zip' });
}
