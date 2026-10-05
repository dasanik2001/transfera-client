const envUploadMb = process.env.NEXT_PUBLIC_MAX_UPLOAD_MB
  ? Number(process.env.NEXT_PUBLIC_MAX_UPLOAD_MB)
  : NaN;

/** Max upload size enforced in the UI (default 500 MB, configurable up to 8192 MB). */
export const MAX_UPLOAD_MB =
  Number.isFinite(envUploadMb) && envUploadMb > 0 ? envUploadMb : 500;
export const MAX_UPLOAD_BYTES = MAX_UPLOAD_MB * 1024 * 1024;

export function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

/** Returns a user-facing error message, or null if the file is allowed. */
export function validateUploadFile(file: File): string | null {
  if (file.size > MAX_UPLOAD_BYTES) {
    return `File must be ${MAX_UPLOAD_MB} MB or smaller (selected: ${formatFileSize(file.size)}).`;
  }
  return null;
}

/** Computes the total byte size of an array of files. */
export function calculateTotalSize(files: File[]): number {
  return files.reduce((acc, file) => acc + file.size, 0);
}

/** Validates that multiple files together do not exceed the MAX_UPLOAD_MB cap. */
export function validateUploadFiles(files: File[]): string | null {
  if (!files || files.length === 0) {
    return 'Please select at least one file.';
  }

  const totalBytes = calculateTotalSize(files);
  if (totalBytes > MAX_UPLOAD_BYTES) {
    return `Total combined size must be ${MAX_UPLOAD_MB} MB or smaller (selected: ${formatFileSize(totalBytes)} across ${files.length} file${files.length === 1 ? '' : 's'}).`;
  }

  return null;
}
