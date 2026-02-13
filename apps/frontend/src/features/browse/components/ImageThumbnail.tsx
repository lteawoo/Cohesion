import React, { useState } from 'react';
import { FileOutlined } from '@ant-design/icons';
import { Spin } from 'antd';

interface ImageThumbnailProps {
  spaceId: number;
  spacePath: string;
  path: string;       // 절대 경로
  alt: string;
  size?: number;
  fit?: 'contain' | 'cover';
}

export const ImageThumbnail: React.FC<ImageThumbnailProps> = ({
  spaceId,
  spacePath,
  path,
  alt,
  size = 120,
  fit = 'cover',
}) => {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  const relativePath = path.replace(spacePath, '').replace(/^\//, '');
  const src = `/api/spaces/${spaceId}/files/download?path=${encodeURIComponent(relativePath)}`;

  if (error) {
    return (
      <div
        style={{
          width: '100%',
          height: size,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          borderRadius: '6px',
          backgroundColor: 'rgba(140, 140, 140, 0.08)',
        }}
      >
        <FileOutlined style={{ fontSize: '48px', color: '#8c8c8c' }} />
      </div>
    );
  }

  return (
    <div
      style={{
        width: '100%',
        height: size,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        position: 'relative',
        overflow: 'hidden',
        borderRadius: '6px',
        backgroundColor: 'rgba(140, 140, 140, 0.08)',
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
          width: '100%',
          height: '100%',
          objectFit: fit,
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
