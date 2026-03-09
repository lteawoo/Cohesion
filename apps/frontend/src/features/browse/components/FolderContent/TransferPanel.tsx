import React from 'react';
import {
  Button,
  Progress,
  Tag,
  Typography,
  theme,
} from 'antd';
import {
  CloseOutlined,
  DownloadOutlined,
  StopOutlined,
  SwapOutlined,
  UploadOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import BottomSheet from '@/components/common/BottomSheet';
import {
  isActiveTransferStatus,
  isClearableTransferStatus,
  useTransferCenterStore,
} from '@/stores/transferCenterStore';
import type { BrowserTransferItem } from '@/stores/transferCenterStore';

interface TransferPanelProps {
  isMobile: boolean;
  onCancelUpload: (transferId: string) => void;
}

function formatTransferName(name: string, maxLength = 32): string {
  if (name.length <= maxLength) {
    return name;
  }

  const extensionIndex = name.lastIndexOf('.');
  const hasVisibleExtension = extensionIndex > 0 && name.length-extensionIndex <= 10;
  if (!hasVisibleExtension) {
    return `${name.slice(0, Math.max(0, maxLength - 3))}...`;
  }

  const namePart = name.slice(0, extensionIndex);
  const extension = name.slice(extensionIndex);
  const visibleNameChars = Math.max(8, maxLength - extension.length - 3);
  if (namePart.length <= visibleNameChars) {
    return name;
  }

  const prefixLength = Math.max(5, Math.ceil(visibleNameChars * 0.7));
  const suffixLength = Math.max(3, visibleNameChars - prefixLength);
  return `${namePart.slice(0, prefixLength)}...${namePart.slice(namePart.length - suffixLength)}${extension}`;
}

function getTransferLabel(
  transfer: BrowserTransferItem,
  t: (key: string) => string
): { color: string; text: string } | null {
  if (transfer.kind === 'download' && transfer.status === 'running') {
    return { color: 'processing', text: t('fileOperations.transferStatusRequesting') };
  }
  switch (transfer.status) {
    case 'uploading':
      return { color: 'processing', text: t('fileOperations.transferStatusUploading') };
    case 'queued':
      return { color: 'default', text: t('fileOperations.transferStatusQueued') };
    case 'running':
      return { color: 'processing', text: t('fileOperations.transferStatusPreparing') };
    case 'ready':
      return { color: 'success', text: t('fileOperations.transferStatusReady') };
    case 'completed':
      return { color: 'success', text: t('fileOperations.transferStatusCompleted') };
    case 'handed_off':
      return { color: 'success', text: t('fileOperations.transferStatusCompleted') };
    case 'failed':
      return { color: 'error', text: t('fileOperations.transferStatusFailed') };
    case 'expired':
      return { color: 'warning', text: t('fileOperations.transferStatusExpired') };
    case 'canceled':
      return { color: 'default', text: t('fileOperations.transferStatusCanceled') };
    default:
      return { color: 'default', text: transfer.status };
  }
}

function getTransferStatusText(transfer: BrowserTransferItem, t: (key: string) => string): string {
  const label = getTransferLabel(transfer, t);
  return label?.text ?? transfer.status;
}

function getTransferProgressPercent(transfer: BrowserTransferItem): number {
  if (transfer.kind === 'upload') {
    return transfer.progressPercent;
  }
  if (transfer.kind === 'download' && typeof transfer.total === 'number' && transfer.total > 0) {
    const loaded = transfer.loaded ?? 0;
    return Math.min(100, Math.round((loaded / transfer.total) * 100));
  }
  if (transfer.kind === 'archive' && transfer.totalItems > 0) {
    return Math.min(100, Math.round((transfer.processedItems / transfer.totalItems) * 100));
  }
  if (transfer.kind === 'archive' && transfer.totalSourceBytes > 0) {
    return Math.min(100, Math.round((transfer.processedSourceBytes / transfer.totalSourceBytes) * 100));
  }
  return transfer.status === 'completed' || transfer.status === 'ready' || transfer.status === 'handed_off' ? 100 : 0;
}

function getTransferProgressPresentation(transfer: BrowserTransferItem): {
  percent: number;
  progressStatus: 'normal' | 'exception' | 'active' | 'success';
  showInfo: boolean;
  indeterminate: boolean;
} {
  const percent = getTransferProgressPercent(transfer);

  if (transfer.status === 'failed' || transfer.status === 'expired' || transfer.status === 'canceled') {
    return {
      percent,
      progressStatus: 'exception',
      showInfo: transfer.kind !== 'download',
      indeterminate: false,
    };
  }

  if (transfer.kind === 'download' && transfer.status === 'running') {
    return {
      percent: 0,
      progressStatus: 'active',
      showInfo: false,
      indeterminate: true,
    };
  }

  if (transfer.status === 'completed' || transfer.status === 'ready' || transfer.status === 'handed_off') {
    return {
      percent,
      progressStatus: 'success',
      showInfo: transfer.kind !== 'download',
      indeterminate: false,
    };
  }

  return {
    percent,
    progressStatus: transfer.status === 'uploading' || transfer.status === 'running' ? 'active' : 'normal',
    showInfo: transfer.kind !== 'download',
    indeterminate: false,
  };
}

function TransferRow({
  transfer,
  onCancelUpload,
  onDismiss,
}: {
  transfer: BrowserTransferItem;
  onCancelUpload: (transferId: string) => void;
  onDismiss: (transferId: string) => void;
}): React.ReactElement {
  const { token } = theme.useToken();
  const { t } = useTranslation();
  const label = getTransferLabel(transfer, t);
  const statusText = getTransferStatusText(transfer, t);
  const canCancel = (
    (transfer.kind === 'upload' && (transfer.status === 'queued' || transfer.status === 'uploading'))
    || (transfer.kind === 'archive' && transfer.status === 'queued')
    || (transfer.kind === 'download' && transfer.status === 'running')
  );
  const canDismiss = !isActiveTransferStatus(transfer.status);
  const progress = getTransferProgressPresentation(transfer);
  const displayName = formatTransferName(transfer.name);
  const transferIcon = transfer.kind === 'upload'
    ? <UploadOutlined style={{ color: token.colorPrimary }} />
    : <DownloadOutlined style={{ color: token.colorPrimary }} />;

  return (
    <div
      key={transfer.id}
      style={{
        padding: 12,
        borderRadius: 12,
        background: token.colorBgContainer,
        border: `1px solid ${token.colorBorderSecondary}`,
      }}
    >
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 12 }}>
        <div style={{ minWidth: 0, flex: 1 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 0 }}>
            <span aria-hidden>{transferIcon}</span>
            <Typography.Text
              strong
              title={transfer.name}
              style={{
                minWidth: 0,
                flex: 1,
                whiteSpace: 'nowrap',
              }}
            >
              {displayName}
            </Typography.Text>
            {label ? <Tag color={label.color}>{label.text}</Tag> : null}
          </div>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          {canCancel ? (
            <Button
              size="small"
              icon={<StopOutlined />}
              onClick={() => onCancelUpload(transfer.id)}
            >
              {t('fileOperations.cancelTransfer')}
            </Button>
          ) : null}
          {canDismiss ? (
            <Button
              size="small"
              type="text"
              icon={<CloseOutlined />}
              onClick={() => onDismiss(transfer.id)}
            />
          ) : null}
        </div>
      </div>

      <div style={{ marginTop: 12 }}>
        {progress.indeterminate ? (
          <div
            aria-busy="true"
            aria-valuetext={statusText}
            role="progressbar"
            style={{
              position: 'relative',
              height: 6,
              borderRadius: 999,
              overflow: 'hidden',
              background: token.colorFillSecondary ?? token.colorBorderSecondary,
            }}
          >
            <div
              style={{
                width: '36%',
                height: '100%',
                borderRadius: 999,
                background: token.colorPrimary,
              }}
            />
          </div>
        ) : (
          <Progress
            percent={progress.percent}
            status={progress.progressStatus}
            showInfo={progress.showInfo}
            size="small"
            format={(percent) => `${percent ?? 0}%`}
          />
        )}
        <Typography.Text type="secondary" style={{ display: 'block', marginTop: 6 }}>
          {statusText}
        </Typography.Text>
        {transfer.message ? (
          <Typography.Text type="secondary" style={{ display: 'block', marginTop: 2 }}>
            {transfer.message}
          </Typography.Text>
        ) : null}
      </div>
    </div>
  );
}

const TransferPanel: React.FC<TransferPanelProps> = ({ isMobile, onCancelUpload }) => {
  const { token } = theme.useToken();
  const { t } = useTranslation();
  const transfers = useTransferCenterStore((state) => state.transfers);
  const isOpen = useTransferCenterStore((state) => state.isOpen);
  const setOpen = useTransferCenterStore((state) => state.setOpen);
  const toggleOpen = useTransferCenterStore((state) => state.toggleOpen);
  const dismissTransfer = useTransferCenterStore((state) => state.dismissTransfer);
  const clearCompletedTransfers = useTransferCenterStore((state) => state.clearCompletedTransfers);

  if (transfers.length === 0) {
    return null;
  }

  const activeCount = transfers.filter((transfer) => isActiveTransferStatus(transfer.status)).length;
  const clearableCount = transfers.filter((transfer) => isClearableTransferStatus(transfer.status)).length;
  const listMaxHeight = isMobile ? '480px' : '420px';

  const panelContent = (
    <div
      data-testid={isMobile ? 'transfer-center-mobile-sheet' : 'transfer-center-panel'}
      style={{
        padding: 16,
        borderRadius: isMobile ? 0 : 16,
        border: isMobile ? undefined : `1px solid ${token.colorBorderSecondary}`,
        background: token.colorBgElevated,
        boxShadow: isMobile ? 'none' : token.boxShadowTertiary,
        display: 'flex',
        flexDirection: 'column',
        minHeight: 0,
      }}
    >
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 12,
          marginBottom: 12,
        }}
      >
        <div style={{ minWidth: 0 }}>
          <Typography.Text strong>{t('fileOperations.transferPanelTitle')}</Typography.Text>
        </div>
        {clearableCount > 0 ? (
          <Button
            data-testid="transfer-center-clear-completed"
            size="small"
            type="text"
            onClick={clearCompletedTransfers}
          >
            {t('fileOperations.transferClearCompleted')}
          </Button>
        ) : null}
      </div>

      <div
        data-testid="transfer-center-list"
        style={{
          display: 'grid',
          gap: 12,
          maxHeight: listMaxHeight,
          overflowY: 'auto',
          overscrollBehavior: 'contain',
          paddingRight: 4,
          minHeight: 0,
        }}
      >
        {transfers.map((transfer) => (
          <TransferRow
            key={transfer.id}
            transfer={transfer}
            onCancelUpload={onCancelUpload}
            onDismiss={dismissTransfer}
          />
        ))}
      </div>
    </div>
  );

  const trigger = (
    <Button
      aria-label={t('fileOperations.transferPanelTitle')}
      data-testid="transfer-center-trigger"
      icon={<SwapOutlined />}
      shape="circle"
      size="large"
      type={activeCount > 0 ? 'primary' : 'default'}
      onClick={isMobile ? () => setOpen(true) : toggleOpen}
      style={{
        width: 48,
        height: 48,
        boxShadow: token.boxShadowSecondary,
      }}
    />
  );

  if (isMobile) {
    return (
      <>
        <div
          style={{
            position: 'fixed',
            right: 16,
            bottom: 16,
            zIndex: 1050,
          }}
        >
          {trigger}
        </div>
        <BottomSheet
          open={isOpen}
          onClose={() => setOpen(false)}
          snapPoints={[0.76, 0.92]}
          initialSnapIndex={1}
        >
          {panelContent}
        </BottomSheet>
      </>
    );
  }

  return (
    <div
      style={{
        position: 'fixed',
        right: 24,
        bottom: 24,
        zIndex: 1050,
        width: 'min(360px, calc(100vw - 32px))',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'flex-end',
        gap: 12,
      }}
    >
      {isOpen ? panelContent : null}
      {trigger}
    </div>
  );
};

export default TransferPanel;
