import React from 'react';
import {
  AudioOutlined,
  CodeOutlined,
  FileExcelOutlined,
  FileOutlined,
  FilePdfOutlined,
  FilePptOutlined,
  FileTextOutlined,
  FileWordOutlined,
  FileZipOutlined,
  VideoCameraOutlined,
} from '@ant-design/icons';
import { getFileCategory } from '../utils/fileTypeUtils';

interface FileTypeIconProps {
  filename: string;
  size?: number;
}

const DEFAULT_ICON_COLOR = 'var(--app-muted-icon-color, #778da9)';

const categoryStyles: Record<string, { color: string }> = {
  pdf: { color: '#cf1322' },
  word: { color: '#1d39c4' },
  excel: { color: '#389e0d' },
  ppt: { color: '#d46b08' },
  archive: { color: '#531dab' },
  audio: { color: '#08979c' },
  video: { color: '#1677ff' },
  code: { color: '#2f54eb' },
  text: { color: '#595959' },
  default: { color: DEFAULT_ICON_COLOR },
};

export const FileTypeIcon: React.FC<FileTypeIconProps> = ({ filename, size = 18 }) => {
  const category = getFileCategory(filename);
  const style = {
    fontSize: size,
    color: categoryStyles[category]?.color ?? DEFAULT_ICON_COLOR,
  };

  switch (category) {
    case 'pdf':
      return <FilePdfOutlined style={style} />;
    case 'word':
      return <FileWordOutlined style={style} />;
    case 'excel':
      return <FileExcelOutlined style={style} />;
    case 'ppt':
      return <FilePptOutlined style={style} />;
    case 'archive':
      return <FileZipOutlined style={style} />;
    case 'audio':
      return <AudioOutlined style={style} />;
    case 'video':
      return <VideoCameraOutlined style={style} />;
    case 'code':
      return <CodeOutlined style={style} />;
    case 'text':
      return <FileTextOutlined style={style} />;
    case 'image':
      return <FileOutlined style={style} />;
    default:
      return <FileOutlined style={style} />;
  }
};
