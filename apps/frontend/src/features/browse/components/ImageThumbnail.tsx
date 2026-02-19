import React, { useState } from 'react';
import { FileOutlined } from '@ant-design/icons';
import { Spin } from 'antd';

interface ImageThumbnailProps {
  spaceId: number;
  path: string;
  alt: string;
  size?: number;
  fit?: 'contain' | 'cover';
}

export const ImageThumbnail: React.FC<ImageThumbnailProps> = ({
  spaceId,
  path,
  alt,
  size = 120,
  fit = 'cover',
}) => {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  const src = `/api/spaces/${spaceId}/files/download?path=${encodeURIComponent(path)}`;

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
          backgroundColor: 'var(--ant-color-fill-tertiary, rgba(119, 141, 169, 0.12))',
        }}
      >
        <FileOutlined style={{ fontSize: '48px', color: 'var(--app-muted-icon-color, #778da9)' }} />
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
        backgroundColor: 'var(--ant-color-fill-tertiary, rgba(119, 141, 169, 0.12))',
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
