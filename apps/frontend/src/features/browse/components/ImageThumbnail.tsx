import React, { useState } from 'react';
import { FileOutlined } from '@ant-design/icons';
import { Spin } from 'antd';

interface ImageThumbnailProps {
  path: string;
  alt: string;
  size?: number; // 컨테이너 크기
}

export const ImageThumbnail: React.FC<ImageThumbnailProps> = ({
  path,
  alt,
  size = 120,
}) => {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  if (error) {
    // 로딩 실패 시 기본 파일 아이콘
    return (
      <div
        style={{
          height: size,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        <FileOutlined style={{ fontSize: '48px', color: '#8c8c8c' }} />
      </div>
    );
  }

  return (
    <div
      style={{
        height: size,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        position: 'relative',
      }}
    >
      {loading && (
        <div style={{ position: 'absolute' }}>
          <Spin />
        </div>
      )}
      <img
        src={`/api/browse/download?path=${encodeURIComponent(path)}`}
        alt={alt}
        loading="lazy"
        style={{
          maxWidth: '100%',
          maxHeight: size,
          objectFit: 'contain',
          borderRadius: '4px',
          opacity: loading ? 0 : 1,
          transition: 'opacity 0.2s',
        }}
        onLoad={() => setLoading(false)}
        onError={() => {
          setLoading(false);
          setError(true);
        }}
      />
    </div>
  );
};
