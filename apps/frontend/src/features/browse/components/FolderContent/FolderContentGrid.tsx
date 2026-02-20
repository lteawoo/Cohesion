import React from 'react';
import { Card, Empty, Grid } from 'antd';
import { FolderFilled } from '@ant-design/icons';
import type { FileNode } from '../../types';
import { formatSize } from '../../constants';
import { isImageFile } from '../../utils/fileTypeUtils';
import { ImageThumbnail } from '../ImageThumbnail';
import { FileTypeIcon } from '../FileTypeIcon';

interface FolderContentGridProps {
  dataSource: FileNode[];
  loading: boolean;
  selectedItems: Set<string>;
  dragOverFolder: string | null;
  spaceId?: number;
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
}) => {
  const screens = Grid.useBreakpoint();
  const isMobile = !screens.lg;
  const gridTemplateColumns = isMobile
    ? 'repeat(2, minmax(0, 1fr))'
    : 'repeat(auto-fill, minmax(172px, 1fr))';

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
                onContextMenu={(e) => onContextMenu(e, item)}
                onDragStart={(e) => onItemDragStart(e, item)}
                onDragEnd={(e) => onItemDragEnd(e)}
                onDragOver={(e) => item.isDir && onFolderDragOver(e, item)}
                onDragLeave={(e) => item.isDir && onFolderDragLeave(e)}
                onDrop={(e) => item.isDir && onFolderDrop(e, item)}
                style={{
                  textAlign: 'center',
                  cursor: 'pointer',
                  backgroundColor: isSelected
                    ? 'var(--browse-selection-bg)'
                    : dragOverFolder === item.path
                      ? 'var(--browse-dragover-bg)'
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
                        backgroundColor: 'var(--ant-color-fill-tertiary, rgba(119, 141, 169, 0.12))',
                      }}
                    >
                      <FolderFilled style={{ fontSize: '48px', color: 'var(--app-folder-icon-color, #415a77)' }} />
                    </div>
                  ) : isImageFile(item.name) && spaceId ? (
                    <ImageThumbnail
                      spaceId={spaceId}
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
                        backgroundColor: 'var(--ant-color-fill-tertiary, rgba(119, 141, 169, 0.12))',
                      }}
                    >
                      <FileTypeIcon filename={item.name} size={48} />
                    </div>
                  )}
                </div>
                <div
                  title={item.name}
                  style={{
                    fontSize: '12px',
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap',
                    marginBottom: '4px',
                  }}
                >
                  {item.name}
                </div>
                {!item.isDir && (
                  <div style={{ fontSize: '11px', color: 'var(--ant-color-text-secondary, #778da9)' }}>
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
