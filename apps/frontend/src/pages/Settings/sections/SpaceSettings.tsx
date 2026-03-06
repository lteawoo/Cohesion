import { ReloadOutlined } from '@ant-design/icons';
import { App, Button, Card, Input, InputNumber, Progress, Space, Table, Tag, Typography } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import type { ChangeEvent } from 'react';
import { apiFetch } from '@/api/client';
import { toApiError } from '@/api/error';
import { useAuth } from '@/features/auth/useAuth';
import { useSpaceStore } from '@/stores/spaceStore';
import SettingSectionHeader from '../components/SettingSectionHeader';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;
const BYTES_PER_MB = 1024 * 1024;

interface SpaceUsageItem {
  spaceId: number;
  spaceName: string;
  usedBytes: number;
  quotaBytes?: number;
  overQuota: boolean;
  scannedAt: string;
}

function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'] as const;
  let value = bytes;
  let index = 0;
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024;
    index += 1;
  }
  const fixed = value >= 100 || index === 0 ? value.toFixed(0) : value.toFixed(1);
  return `${fixed} ${units[index]}`;
}

const SpaceSettings = () => {
  const { t } = useTranslation();
  const { message, modal } = App.useApp();
  const { user } = useAuth();
  const [usageLoading, setUsageLoading] = useState(false);
  const [spaceUsages, setSpaceUsages] = useState<SpaceUsageItem[]>([]);
  const [nameDrafts, setNameDrafts] = useState<Record<number, string>>({});
  const [quotaDrafts, setQuotaDrafts] = useState<Record<number, number | null>>({});
  const [rowSaving, setRowSaving] = useState<Record<number, boolean>>({});
  const fetchSpaces = useSpaceStore((state) => state.fetchSpaces);
  const renameSpace = useSpaceStore((state) => state.renameSpace);
  const deleteSpace = useSpaceStore((state) => state.deleteSpace);

  const permissions = user?.permissions ?? [];
  const canReadSpaceSettings = permissions.includes('space.read');
  const canWriteSpaceSettings = permissions.includes('space.write');

  const loadSpaceUsage = useCallback(async () => {
    setUsageLoading(true);
    try {
      const response = await apiFetch('/api/spaces/usage');
      if (!response.ok) {
        throw await toApiError(response, t('spaceSettings.loadUsageFallback'));
      }
      const data = (await response.json()) as SpaceUsageItem[];
      setSpaceUsages(data);
      setNameDrafts(
        data.reduce<Record<number, string>>((acc, item) => {
          acc[item.spaceId] = item.spaceName;
          return acc;
        }, {})
      );
      setQuotaDrafts(
        data.reduce<Record<number, number | null>>((acc, item) => {
          acc[item.spaceId] = typeof item.quotaBytes === 'number'
            ? Number((item.quotaBytes / BYTES_PER_MB).toFixed(2))
            : null;
          return acc;
        }, {})
      );
    } catch {
      message.error(t('spaceSettings.loadUsageFailed'));
    } finally {
      setUsageLoading(false);
    }
  }, [message, t]);

  const refreshSpaceSettings = useCallback(async () => {
    await Promise.all([
      loadSpaceUsage(),
      fetchSpaces(),
    ]);
  }, [fetchSpaces, loadSpaceUsage]);

  useEffect(() => {
    if (!canReadSpaceSettings) {
      return;
    }
    void refreshSpaceSettings();
  }, [canReadSpaceSettings, refreshSpaceSettings]);

  const handleNameDraftChange = (spaceId: number, value: string) => {
    setNameDrafts((prev) => ({
      ...prev,
      [spaceId]: value,
    }));
  };

  const handleQuotaDraftChange = (spaceId: number, value: number | null) => {
    setQuotaDrafts((prev) => ({
      ...prev,
      [spaceId]: value,
    }));
  };

  const handleSaveRow = async (item: SpaceUsageItem) => {
    if (!canWriteSpaceSettings) {
      message.error(t('spaceSettings.noWritePermission'));
      return;
    }

    const nextName = (nameDrafts[item.spaceId] ?? item.spaceName).trim();
    if (nextName === '') {
      message.error(t('spaceSettings.invalidName'));
      return;
    }

    const draftMb = quotaDrafts[item.spaceId];
    const quotaBytes = typeof draftMb === 'number'
      ? Math.max(0, Math.round(draftMb * BYTES_PER_MB))
      : null;
    const currentQuotaBytes = item.quotaBytes ?? null;
    const nameChanged = nextName !== item.spaceName;
    const quotaChanged = quotaBytes !== currentQuotaBytes;

    if (!nameChanged && !quotaChanged) {
      return;
    }

    let hasMutation = false;
    setRowSaving((prev) => ({ ...prev, [item.spaceId]: true }));
    try {
      if (nameChanged) {
        await renameSpace(item.spaceId, nextName);
        hasMutation = true;
      }

      if (quotaChanged) {
        const response = await apiFetch(`/api/spaces/${item.spaceId}/quota`, {
          method: 'PATCH',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ quotaBytes }),
        });
        if (!response.ok) {
          throw await toApiError(response, t('spaceSettings.saveSpaceFallback'));
        }
        hasMutation = true;
      }

      await refreshSpaceSettings();
      message.success(t('spaceSettings.saveSpaceSuccess'));
    } catch {
      if (hasMutation) {
        await refreshSpaceSettings();
      }
      message.error(t('spaceSettings.saveSpaceFailed'));
    } finally {
      setRowSaving((prev) => ({ ...prev, [item.spaceId]: false }));
    }
  };

  const handleDeleteSpace = (item: SpaceUsageItem) => {
    if (!canWriteSpaceSettings) {
      message.error(t('spaceSettings.noWritePermission'));
      return;
    }

    modal.confirm({
      title: t('spaceSettings.deleteSpaceTitle'),
      content: t('spaceSettings.deleteSpaceDescription', { spaceName: item.spaceName }),
      okText: t('spaceSettings.deleteSpaceAction'),
      cancelText: t('spaceSettings.cancel'),
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await deleteSpace(item.spaceId);
          message.success(t('spaceSettings.deleteSpaceSuccess'));
          await loadSpaceUsage();
        } catch {
          message.error(t('spaceSettings.deleteSpaceFailed'));
        }
      },
    });
  };

  if (!canReadSpaceSettings) {
    return <div>{t('spaceSettings.noPermission')}</div>;
  }

  const spaceUsageColumns = [
    {
      title: t('spaceSettings.columnName'),
      key: 'name',
      render: (_: unknown, item: SpaceUsageItem) => (
        canWriteSpaceSettings ? (
          <Input
            size="small"
            value={nameDrafts[item.spaceId] ?? item.spaceName}
            onChange={(event: ChangeEvent<HTMLInputElement>) => handleNameDraftChange(item.spaceId, event.target.value)}
            onPressEnter={() => void handleSaveRow(item)}
            placeholder={t('spaceSettings.spaceNamePlaceholder')}
            disabled={rowSaving[item.spaceId]}
            style={{ minWidth: 180 }}
          />
        ) : (
          <Space size={8}>
            <Text strong>{item.spaceName}</Text>
            {item.overQuota ? <Tag color="error">{t('spaceSettings.overQuotaTag')}</Tag> : null}
          </Space>
        )
      ),
    },
    {
      title: t('spaceSettings.columnUsage'),
      key: 'usedBytes',
      render: (_: unknown, item: SpaceUsageItem) => <Text>{formatBytes(item.usedBytes)}</Text>,
    },
    {
      title: t('spaceSettings.columnQuota'),
      key: 'quotaEditor',
      render: (_: unknown, item: SpaceUsageItem) => (
        canWriteSpaceSettings ? (
          <Space size={8} wrap>
            <InputNumber
              size="small"
              min={0}
              step={1}
              value={quotaDrafts[item.spaceId]}
              onChange={(value: number | null) => handleQuotaDraftChange(item.spaceId, value)}
              placeholder={t('spaceSettings.unlimited')}
              disabled={rowSaving[item.spaceId]}
            />
            <Text type="secondary">MB</Text>
          </Space>
        ) : (
          item.quotaBytes != null ? <Text>{formatBytes(item.quotaBytes)}</Text> : <Tag>{t('spaceSettings.unlimited')}</Tag>
        )
      ),
    },
    {
      title: t('spaceSettings.columnUsageRate'),
      key: 'usageRate',
      render: (_: unknown, item: SpaceUsageItem) => {
        if (item.quotaBytes == null || item.quotaBytes <= 0) {
          return <Text type="secondary">{t('spaceSettings.unlimited')}</Text>;
        }
        const ratio = (item.usedBytes / item.quotaBytes) * 100;
        return (
          <Progress
            percent={Math.min(100, Math.round(ratio))}
            status={ratio > 100 ? 'exception' : 'normal'}
            size="small"
            style={{ minWidth: 140 }}
          />
        );
      },
    },
  ];

  if (canWriteSpaceSettings) {
    spaceUsageColumns.push({
      title: t('spaceSettings.columnActions'),
      key: 'actions',
      render: (_: unknown, item: SpaceUsageItem) => {
        const draftName = (nameDrafts[item.spaceId] ?? item.spaceName).trim();
        const draftMb = quotaDrafts[item.spaceId];
        const quotaBytes = typeof draftMb === 'number'
          ? Math.max(0, Math.round(draftMb * BYTES_PER_MB))
          : null;
        const hasChanges = draftName !== item.spaceName || quotaBytes !== (item.quotaBytes ?? null);

        return (
          <Space size={8} wrap>
            <Button
              size="small"
              type="primary"
              loading={rowSaving[item.spaceId]}
              onClick={() => void handleSaveRow(item)}
              disabled={rowSaving[item.spaceId] || draftName === '' || !hasChanges}
            >
              {t('spaceSettings.saveAction')}
            </Button>
            <Button
              size="small"
              danger
              disabled={rowSaving[item.spaceId]}
              onClick={() => handleDeleteSpace(item)}
            >
              {t('spaceSettings.deleteSpaceAction')}
            </Button>
          </Space>
        );
      },
    });
  }

  return (
    <Space orientation="vertical" size="middle" className="settings-section" style={{ width: '100%' }}>
      <SettingSectionHeader title={t('spaceSettings.sectionTitle')} subtitle={t('spaceSettings.sectionSubtitle')} />

      <Card
        size="small"
        extra={(
          <Button size="small" icon={<ReloadOutlined />} onClick={() => void refreshSpaceSettings()} loading={usageLoading}>
            {t('spaceSettings.refresh')}
          </Button>
        )}
      >
        <Space orientation="vertical" size="middle" style={{ width: '100%' }}>
          <Text type="secondary">{t('spaceSettings.quotaDescription')}</Text>
          <Table<SpaceUsageItem>
            size="small"
            rowKey={(item: SpaceUsageItem) => item.spaceId}
            loading={usageLoading}
            columns={spaceUsageColumns}
            dataSource={spaceUsages}
            pagination={false}
            locale={{ emptyText: t('spaceSettings.emptyText') }}
          />
        </Space>
      </Card>
    </Space>
  );
};

export default SpaceSettings;
