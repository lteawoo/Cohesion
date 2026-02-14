import React from 'react';
import { Card, Empty, Grid } from 'antd';
import { FolderFilled, FileOutlined } from '@ant-design/icons';
import type { FileNode } from '../../types';
import { formatSize } from '../../constants';
import { isImageFile } from '../../utils/fileTypeUtils';
import { ImageThumbnail } from '../ImageThumbnail';

interface FolderContentGridProps {
  dataSource: FileNode[];
  loading: boolean;
  selectedItems: Set<string>;
  dragOverFolder: string | null;
  spaceId?: number;
  spacePath?: string;
  onItemClick: (e: React.MouseEvent<HTMLElement>, record: FileNode, index: number) => void;
  onItemDoubleClick: (path: string) => void;
  onItemTouchStart?: (record: FileNode, index: number) => void;
  onItemTouchEnd?: () => void;
  onItemTouchCancel?: () => void;
  onContextMenu: (e: React.MouseEvent<HTMLElement>, record: FileNode) => void;
  onItemDragStart: (e: React.DragEvent<HTMLElement>, record: FileNode) => void;
  onItemDragEnd: (e: React.DragEvent<HTMLElement>) => void;
  onFolderDragOver: (e: React.DragEvent<HTMLElement>, record: FileNode) => void;
  onFolderDragLeave: (e: React.DragEvent<HTMLElement>) => void;
  onFolderDrop: (e: React.DragEvent<HTMLElement>, record: FileNode) => void;
  itemsRef?: React.MutableRefObject<Map<string, HTMLElement>>;
  disableDraggable?: boolean;
}

const FolderContentGrid: React.FC<FolderContentGridProps> = ({
  dataSource,
  loading,
  selectedItems,
  dragOverFolder,
  onItemClick,
  onItemDoubleClick,
  onItemTouchStart,
  onItemTouchEnd,
  onItemTouchCancel,
  onContextMenu,
  onItemDragStart,
  onItemDragEnd,
  onFolderDragOver,
  onFolderDragLeave,
  onFolderDrop,
  itemsRef,
  disableDraggable = false,
  spaceId,
  spacePath,
}) => {
  const screens = Grid.useBreakpoint();
  const isMobile = !screens.sm;
  const gridTemplateColumns = isMobile
    ? 'repeat(2, minmax(0, 1fr))'
    : 'repeat(auto-fit, minmax(172px, 220px))';

  return (
    <>
      {dataSource.length === 0 && !loading ? (
        <Empty description="이 폴더는 비어 있습니다." />
      ) : (
        <div
          style={{
            display: 'grid',
            gridTemplateColumns,
            justifyContent: 'start',
            gap: '12px',
          }}
        >
          {dataSource.map((item, index) => {
            const isSelected = selectedItems.has(item.path);
            return (
              <Card
                key={item.path}
                ref={(el) => {
                  if (el && itemsRef) {
                    itemsRef.current.set(item.path, el);
                  }
                }}
                hoverable
                draggable={!disableDraggable}
                onClick={(e) => onItemClick(e, item, index)}
                onDoubleClick={() => item.isDir && onItemDoubleClick(item.path)}
                onTouchStart={() => onItemTouchStart?.(item, index)}
                onTouchEnd={() => onItemTouchEnd?.()}
                onTouchCancel={() => onItemTouchCancel?.()}
                onTouchMove={() => onItemTouchCancel?.()}
                onContextMenu={(e) => onContextMenu(e, item)}
                onDragStart={(e) => onItemDragStart(e, item)}
                onDragEnd={(e) => onItemDragEnd(e)}
                onDragOver={(e) => item.isDir && onFolderDragOver(e, item)}
                onDragLeave={(e) => item.isDir && onFolderDragLeave(e)}
                onDrop={(e) => item.isDir && onFolderDrop(e, item)}
                style={{
                  textAlign: 'center',
                  cursor: 'pointer',
                  border: isSelected
                    ? '2px solid #1890ff'
                    : dragOverFolder === item.path
                      ? '2px dashed #1890ff'
                      : undefined,
                  backgroundColor: isSelected
                    ? 'rgba(24, 144, 255, 0.1)'
                    : dragOverFolder === item.path
                      ? 'rgba(24, 144, 255, 0.05)'
                      : undefined,
                  userSelect: 'none',
                }}
                styles={{ body: { padding: '12px 8px' } }}
              >
                <div style={{ marginBottom: '6px' }}>
                  {item.isDir ? (
                    <div
                      style={{
                        height: '128px',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        borderRadius: '6px',
                        backgroundColor: 'rgba(140, 140, 140, 0.08)',
                      }}
                    >
                      <FolderFilled style={{ fontSize: '48px', color: '#ffca28' }} />
                    </div>
                  ) : isImageFile(item.name) && spaceId && spacePath ? (
                    <ImageThumbnail
                      spaceId={spaceId}
                      spacePath={spacePath}
                      path={item.path}
                      alt={item.name}
                      size={128}
                      fit="cover"
                    />
                  ) : (
                    <div
                      style={{
                        height: '128px',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        borderRadius: '6px',
                        backgroundColor: 'rgba(140, 140, 140, 0.08)',
                      }}
                    >
                      <FileOutlined style={{ fontSize: '48px', color: '#8c8c8c' }} />
                    </div>
                  )}
                </div>
                <div
                  style={{
                    fontSize: '12px',
                    wordBreak: 'break-word',
                    marginBottom: '4px',
                  }}
                >
                  {item.name}
                </div>
                {!item.isDir && (
                  <div style={{ fontSize: '11px', color: '#8c8c8c' }}>
                    {formatSize(item.size)}
                  </div>
                )}
              </Card>
            );
          })}
        </div>
      )}
    </>
  );
};

export default FolderContentGrid;
