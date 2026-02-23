import React, { useMemo } from 'react';
import { Modal, Table, Space as AntSpace, Button, Empty } from 'antd';
import type { TableColumnsType } from 'antd';
import { DeleteOutlined, RollbackOutlined, FolderFilled, FileOutlined } from '@ant-design/icons';
import type { TrashItem } from '../../hooks/useFileOperations';
import { formatDate, formatSize } from '../../constants';

interface TrashModalProps {
  open: boolean;
  spaceName?: string;
  items: TrashItem[];
  selectedIds: number[];
  loading: boolean;
  processing: boolean;
  onSelectionChange: (ids: number[]) => void;
  onRestore: () => void;
  onDelete: () => void;
  onEmpty: () => void;
  onClose: () => void;
}

const TrashModal: React.FC<TrashModalProps> = ({
  open,
  spaceName,
  items,
  selectedIds,
  loading,
  processing,
  onSelectionChange,
  onRestore,
  onDelete,
  onEmpty,
  onClose,
}) => {
  const columns = useMemo<TableColumnsType<TrashItem>>(() => {
    return [
      {
        title: '이름',
        dataIndex: 'itemName',
        key: 'itemName',
        ellipsis: true,
        render: (_, record) => (
          <AntSpace>
            {record.isDir ? <FolderFilled style={{ color: 'var(--app-folder-icon-color, #415a77)' }} /> : <FileOutlined />}
            <span>{record.itemName}</span>
          </AntSpace>
        ),
      },
      {
        title: '원래 경로',
        dataIndex: 'originalPath',
        key: 'originalPath',
        ellipsis: true,
      },
      {
        title: '삭제 시각',
        dataIndex: 'deletedAt',
        key: 'deletedAt',
        width: 180,
        render: (value: string) => formatDate(value),
      },
      {
        title: '삭제자',
        dataIndex: 'deletedBy',
        key: 'deletedBy',
        width: 120,
      },
      {
        title: '크기',
        dataIndex: 'itemSize',
        key: 'itemSize',
        width: 120,
        align: 'right',
        render: (size: number, record) => (record.isDir ? '-' : formatSize(size)),
      },
    ];
  }, []);

  const hasSelection = selectedIds.length > 0;
  const hasItems = items.length > 0;

  return (
    <Modal
      title={spaceName ? `휴지통 - ${spaceName}` : '휴지통'}
      open={open}
      onCancel={onClose}
      maskClosable={!processing}
      width={960}
      destroyOnClose
      styles={{ mask: { pointerEvents: 'auto' } }}
      footer={[
        <Button
          key="restore"
          icon={<RollbackOutlined />}
          onClick={onRestore}
          disabled={!hasSelection || loading || processing}
        >
          복원
        </Button>,
        <Button
          key="delete"
          icon={<DeleteOutlined />}
          danger
          onClick={onDelete}
          disabled={!hasSelection || loading || processing}
        >
          영구 삭제
        </Button>,
        <Button
          key="empty"
          danger
          onClick={onEmpty}
          disabled={!hasItems || loading || processing}
        >
          비우기
        </Button>,
        <Button key="close" onClick={onClose} disabled={processing}>
          닫기
        </Button>,
      ]}
    >
      <Table<TrashItem>
        size="small"
        rowKey="id"
        dataSource={items}
        columns={columns}
        loading={loading || processing}
        pagination={{ pageSize: 10, showSizeChanger: false }}
        rowSelection={{
          selectedRowKeys: selectedIds,
          onChange: (nextKeys) => {
            onSelectionChange(nextKeys.map((key) => Number(key)));
          },
        }}
        scroll={{ y: 420 }}
        locale={{ emptyText: <Empty description="휴지통이 비어 있습니다." /> }}
      />
    </Modal>
  );
};

export default TrashModal;
