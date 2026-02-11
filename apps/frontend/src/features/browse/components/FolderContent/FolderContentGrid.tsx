import React from 'react';
import { Row, Col, Card, Empty } from 'antd';
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
  onItemClick: (e: React.MouseEvent<HTMLElement>, record: FileNode, index: number) => void;
  onItemDoubleClick: (path: string) => void;
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
  onContextMenu,
  onItemDragStart,
  onItemDragEnd,
  onFolderDragOver,
  onFolderDragLeave,
  onFolderDrop,
  itemsRef,
  disableDraggable = false,
}) => {
  return (
    <Row gutter={[16, 16]}>
      {dataSource.length === 0 && !loading ? (
        <Col span={24}>
          <Empty description="이 폴더는 비어 있습니다." />
        </Col>
      ) : (
        dataSource.map((item, index) => {
          const isSelected = selectedItems.has(item.path);
          return (
            <Col key={item.path} xs={12} sm={8} md={6} lg={4} xl={3}>
              <Card
                ref={(el) => {
                  if (el && itemsRef) {
                    itemsRef.current.set(item.path, el);
                  }
                }}
                hoverable
                draggable={!disableDraggable}
                onClick={(e) => onItemClick(e, item, index)}
                onDoubleClick={() => item.isDir && onItemDoubleClick(item.path)}
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
                styles={{ body: { padding: '16px 8px' } }}
              >
                <div style={{ marginBottom: '8px' }}>
                  {item.isDir ? (
                    <div
                      style={{
                        height: '120px',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                      }}
                    >
                      <FolderFilled style={{ fontSize: '48px', color: '#ffca28' }} />
                    </div>
                  ) : isImageFile(item.name) ? (
                    <ImageThumbnail path={item.path} alt={item.name} size={120} />
                  ) : (
                    <div
                      style={{
                        height: '120px',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
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
            </Col>
          );
        })
      )}
    </Row>
  );
};

export default FolderContentGrid;
