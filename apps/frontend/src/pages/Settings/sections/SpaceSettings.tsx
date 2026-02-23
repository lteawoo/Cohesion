import { ReloadOutlined } from '@ant-design/icons';
import { App, Button, Card, InputNumber, Progress, Space, Table, Tag, Typography } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import { apiFetch } from '@/api/client';
import { toApiError } from '@/api/error';
import { useAuth } from '@/features/auth/useAuth';
import SettingSectionHeader from '../components/SettingSectionHeader';

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
  const { message } = App.useApp();
  const { user } = useAuth();
  const [usageLoading, setUsageLoading] = useState(false);
  const [spaceUsages, setSpaceUsages] = useState<SpaceUsageItem[]>([]);
  const [quotaDrafts, setQuotaDrafts] = useState<Record<number, number | null>>({});
  const [quotaSaving, setQuotaSaving] = useState<Record<number, boolean>>({});

  const permissions = user?.permissions ?? [];
  const canReadSpaceSettings = permissions.includes('space.read') || permissions.includes('space.write');
  const canWriteSpaceSettings = permissions.includes('space.write');

  const loadSpaceUsage = useCallback(async () => {
    setUsageLoading(true);
    try {
      const response = await apiFetch('/api/spaces/usage');
      if (!response.ok) {
        throw await toApiError(response, 'Space 사용량을 불러오지 못했습니다.');
      }
      const data = (await response.json()) as SpaceUsageItem[];
      setSpaceUsages(data);
      setQuotaDrafts(
        data.reduce<Record<number, number | null>>((acc, item) => {
          acc[item.spaceId] = typeof item.quotaBytes === 'number'
            ? Number((item.quotaBytes / BYTES_PER_MB).toFixed(2))
            : null;
          return acc;
        }, {})
      );
    } catch {
      message.error('Space 사용량을 불러오는데 실패했습니다');
    } finally {
      setUsageLoading(false);
    }
  }, [message]);

  useEffect(() => {
    if (!canReadSpaceSettings) {
      return;
    }
    void loadSpaceUsage();
  }, [canReadSpaceSettings, loadSpaceUsage]);

  const handleQuotaDraftChange = (spaceId: number, value: number | null) => {
    setQuotaDrafts((prev) => ({
      ...prev,
      [spaceId]: value,
    }));
  };

  const handleSaveQuota = async (spaceId: number, overrideDraftMb?: number | null) => {
    if (!canWriteSpaceSettings) {
      message.error('Space 쿼터 수정 권한이 없습니다');
      return;
    }

    const draftMb = overrideDraftMb !== undefined ? overrideDraftMb : quotaDrafts[spaceId];
    const quotaBytes = typeof draftMb === 'number'
      ? Math.max(0, Math.round(draftMb * BYTES_PER_MB))
      : null;

    setQuotaSaving((prev) => ({ ...prev, [spaceId]: true }));
    try {
      const response = await apiFetch(`/api/spaces/${spaceId}/quota`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ quotaBytes }),
      });
      if (!response.ok) {
        throw await toApiError(response, 'Space 쿼터를 저장하지 못했습니다.');
      }
      message.success('Space 쿼터가 저장되었습니다');
      await loadSpaceUsage();
    } catch {
      message.error('Space 쿼터 저장에 실패했습니다');
    } finally {
      setQuotaSaving((prev) => ({ ...prev, [spaceId]: false }));
    }
  };

  if (!canReadSpaceSettings) {
    return <div>권한이 없습니다.</div>;
  }

  const spaceUsageColumns = [
    {
      title: 'Space',
      key: 'spaceName',
      render: (_: unknown, item: SpaceUsageItem) => (
        <Space size={8}>
          <Text strong>{item.spaceName}</Text>
          {item.overQuota ? <Tag color="error">초과</Tag> : null}
        </Space>
      ),
    },
    {
      title: '사용량',
      key: 'usedBytes',
      render: (_: unknown, item: SpaceUsageItem) => <Text>{formatBytes(item.usedBytes)}</Text>,
    },
    {
      title: '쿼터',
      key: 'quotaBytes',
      render: (_: unknown, item: SpaceUsageItem) => (
        item.quotaBytes != null ? <Text>{formatBytes(item.quotaBytes)}</Text> : <Tag>무제한</Tag>
      ),
    },
    {
      title: '사용률',
      key: 'usageRate',
      render: (_: unknown, item: SpaceUsageItem) => {
        if (item.quotaBytes == null || item.quotaBytes <= 0) {
          return <Text type="secondary">무제한</Text>;
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
    {
      title: '쿼터 설정',
      key: 'quotaEditor',
      render: (_: unknown, item: SpaceUsageItem) => (
        <Space size={8} wrap>
          <InputNumber
            size="small"
            min={0}
            step={1}
            value={quotaDrafts[item.spaceId]}
            onChange={(value: number | null) => handleQuotaDraftChange(item.spaceId, value)}
            placeholder="무제한"
            addonAfter="MB"
            disabled={!canWriteSpaceSettings || quotaSaving[item.spaceId]}
          />
          <Button
            size="small"
            type="primary"
            loading={quotaSaving[item.spaceId]}
            onClick={() => void handleSaveQuota(item.spaceId)}
            disabled={!canWriteSpaceSettings}
          >
            저장
          </Button>
          <Button
            size="small"
            onClick={() => {
              handleQuotaDraftChange(item.spaceId, null);
              void handleSaveQuota(item.spaceId, null);
            }}
            disabled={!canWriteSpaceSettings || quotaSaving[item.spaceId]}
          >
            무제한
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <Space vertical size="small" className="settings-section">
      <SettingSectionHeader title="스페이스 설정" subtitle="스페이스 사용량과 쿼터를 관리합니다" />

      <Card
        title="Space 쿼터/사용량"
        size="small"
        extra={(
          <Button size="small" icon={<ReloadOutlined />} onClick={() => void loadSpaceUsage()} loading={usageLoading}>
            새로고침
          </Button>
        )}
      >
        <Table<SpaceUsageItem>
          size="small"
          rowKey={(item) => item.spaceId}
          loading={usageLoading}
          columns={spaceUsageColumns}
          dataSource={spaceUsages}
          pagination={false}
          locale={{ emptyText: '표시할 Space가 없습니다' }}
        />
      </Card>
    </Space>
  );
};

export default SpaceSettings;
