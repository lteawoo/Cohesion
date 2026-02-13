import React, { useState } from 'react';
import { FileOutlined } from '@ant-design/icons';
import { Spin } from 'antd';

interface ImageThumbnailProps {
  spaceId: number;
  spacePath: string;
  path: string;       // 절대 경로
  alt: string;
  size?: number;
}

export const ImageThumbnail: React.FC<ImageThumbnailProps> = ({
  spaceId,
  spacePath,
  path,
  alt,
  size = 120,
}) => {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  const relativePath = path.replace(spacePath, '').replace(/^\//, '');
  const src = `/api/spaces/${spaceId}/files/download?path=${encodeURIComponent(relativePath)}`;

  if (error) {
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
        src={src}
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
