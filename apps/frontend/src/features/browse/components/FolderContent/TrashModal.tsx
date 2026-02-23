import React, { useMemo } from 'react';
import { Modal, Table, Space as AntSpace, Button, Empty } from 'antd';
import type { TableColumnsType } from 'antd';
import { DeleteOutlined, RollbackOutlined, FolderFilled, FileOutlined } from '@ant-design/icons';
import type { TrashItem } from '../../hooks/useFileOperations';
import { formatDate, formatSize } from '../../constants';
import { useTranslation } from 'react-i18next';

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
  const { t } = useTranslation();
  const columns = useMemo<TableColumnsType<TrashItem>>(() => {
    return [
      {
        title: t('trashModal.name'),
        dataIndex: 'itemName',
        key: 'itemName',
        ellipsis: true,
        render: (_: unknown, record: TrashItem) => (
          <AntSpace>
            {record.isDir ? <FolderFilled style={{ color: 'var(--app-folder-icon-color, #415a77)' }} /> : <FileOutlined />}
            <span>{record.itemName}</span>
          </AntSpace>
        ),
      },
      {
        title: t('trashModal.originalPath'),
        dataIndex: 'originalPath',
        key: 'originalPath',
        ellipsis: true,
      },
      {
        title: t('trashModal.deletedAt'),
        dataIndex: 'deletedAt',
        key: 'deletedAt',
        width: 180,
        render: (value: string) => formatDate(value),
      },
      {
        title: t('trashModal.deletedBy'),
        dataIndex: 'deletedBy',
        key: 'deletedBy',
        width: 120,
      },
      {
        title: t('trashModal.size'),
        dataIndex: 'itemSize',
        key: 'itemSize',
        width: 120,
        align: 'right',
        render: (size: number, record: TrashItem) => (record.isDir ? '-' : formatSize(size)),
      },
    ];
  }, [t]);

  const hasSelection = selectedIds.length > 0;
  const hasItems = items.length > 0;

  return (
    <Modal
      title={spaceName ? t('trashModal.titleWithSpace', { spaceName }) : t('trashModal.title')}
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
          {t('trashModal.restore')}
        </Button>,
        <Button
          key="delete"
          icon={<DeleteOutlined />}
          danger
          onClick={onDelete}
          disabled={!hasSelection || loading || processing}
        >
          {t('trashModal.permanentDelete')}
        </Button>,
        <Button
          key="empty"
          danger
          onClick={onEmpty}
          disabled={!hasItems || loading || processing}
        >
          {t('trashModal.empty')}
        </Button>,
        <Button key="close" onClick={onClose} disabled={processing}>
          {t('trashModal.close')}
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
        locale={{ emptyText: <Empty description={t('trashModal.emptyText')} /> }}
      />
    </Modal>
  );
};

export default TrashModal;
