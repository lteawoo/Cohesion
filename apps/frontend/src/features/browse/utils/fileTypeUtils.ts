// 이미지 확장자 목록
export const IMAGE_EXTENSIONS = [
  'jpg',
  'jpeg',
  'png',
  'gif',
  'webp',
  'svg',
  'bmp',
  'ico',
];

// 이미지 파일 여부 확인
export const isImageFile = (filename: string): boolean => {
  const ext = filename.split('.').pop()?.toLowerCase();
  return ext ? IMAGE_EXTENSIONS.includes(ext) : false;
};

// MIME 타입 추론 (향후 백엔드 연동 대비)
export const getImageMimeType = (filename: string): string | null => {
  const ext = filename.split('.').pop()?.toLowerCase();
  const mimeMap: Record<string, string> = {
    jpg: 'image/jpeg',
    jpeg: 'image/jpeg',
    png: 'image/png',
    gif: 'image/gif',
    webp: 'image/webp',
    svg: 'image/svg+xml',
    bmp: 'image/bmp',
  };
  return ext && mimeMap[ext] ? mimeMap[ext] : null;
};

// 타입 정의
export type ImageExtension = (typeof IMAGE_EXTENSIONS)[number];
