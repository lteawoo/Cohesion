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

const OFFICE_WORD_EXTENSIONS = ['doc', 'docx', 'odt', 'rtf'];
const OFFICE_EXCEL_EXTENSIONS = ['xls', 'xlsx', 'ods', 'csv'];
const OFFICE_PPT_EXTENSIONS = ['ppt', 'pptx', 'odp'];
const ARCHIVE_EXTENSIONS = ['zip', 'rar', '7z', 'tar', 'gz', 'bz2', 'xz', 'tgz'];
const AUDIO_EXTENSIONS = ['mp3', 'wav', 'flac', 'aac', 'ogg', 'm4a'];
const VIDEO_EXTENSIONS = ['mp4', 'mov', 'mkv', 'avi', 'webm', 'm4v'];
const CODE_EXTENSIONS = [
  'js', 'jsx', 'ts', 'tsx', 'json', 'xml', 'html', 'css', 'scss', 'sass',
  'go', 'py', 'java', 'c', 'cpp', 'h', 'hpp', 'rs', 'sh', 'bash', 'zsh',
  'yml', 'yaml', 'toml', 'ini', 'sql', 'php', 'rb', 'swift', 'kt',
];
const TEXT_EXTENSIONS = ['txt', 'log', 'md', 'markdown'];

export type FileCategory =
  | 'image'
  | 'pdf'
  | 'word'
  | 'excel'
  | 'ppt'
  | 'archive'
  | 'audio'
  | 'video'
  | 'code'
  | 'text'
  | 'default';

export const getFileExtension = (filename: string): string => {
  const ext = filename.split('.').pop()?.toLowerCase();
  return ext ?? '';
};

// 이미지 파일 여부 확인
export const isImageFile = (filename: string): boolean => {
  const ext = getFileExtension(filename);
  return ext !== '' && IMAGE_EXTENSIONS.includes(ext);
};

export const getFileCategory = (filename: string): FileCategory => {
  const ext = getFileExtension(filename);
  if (!ext) return 'default';

  if (IMAGE_EXTENSIONS.includes(ext)) return 'image';
  if (ext === 'pdf') return 'pdf';
  if (OFFICE_WORD_EXTENSIONS.includes(ext)) return 'word';
  if (OFFICE_EXCEL_EXTENSIONS.includes(ext)) return 'excel';
  if (OFFICE_PPT_EXTENSIONS.includes(ext)) return 'ppt';
  if (ARCHIVE_EXTENSIONS.includes(ext)) return 'archive';
  if (AUDIO_EXTENSIONS.includes(ext)) return 'audio';
  if (VIDEO_EXTENSIONS.includes(ext)) return 'video';
  if (CODE_EXTENSIONS.includes(ext)) return 'code';
  if (TEXT_EXTENSIONS.includes(ext)) return 'text';

  return 'default';
};

// MIME 타입 추론 (향후 백엔드 연동 대비)
export const getImageMimeType = (filename: string): string | null => {
  const ext = getFileExtension(filename);
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
