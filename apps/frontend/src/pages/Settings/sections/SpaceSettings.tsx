import { ReloadOutlined } from '@ant-design/icons';
import { App, Button, Card, Input, InputNumber, Progress, Select, Space, Table, Tag, Typography } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import type { ChangeEvent } from 'react';
import {
  type AccountUser,
  type SpaceMember,
  type SpacePermission,
  type UserSpacePermission,
  listAccounts,
  listSpaceMembers,
  updateSpaceMembers,
} from '@/api/accounts';
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

interface SpaceMemberRow {
  id: number;
  username: string;
  nickname: string;
  role: string;
  assignedPermission?: SpacePermission;
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
  const [memberLoading, setMemberLoading] = useState(false);
  const [memberSaving, setMemberSaving] = useState(false);
  const [memberAccounts, setMemberAccounts] = useState<AccountUser[]>([]);
  const [spaceMembers, setSpaceMembers] = useState<SpaceMember[]>([]);
  const [selectedMemberSpaceId, setSelectedMemberSpaceId] = useState<number | null>(null);
  const [memberPermissionMap, setMemberPermissionMap] = useState<Record<number, SpacePermission | undefined>>({});
  const fetchSpaces = useSpaceStore((state) => state.fetchSpaces);
  const renameSpace = useSpaceStore((state) => state.renameSpace);
  const deleteSpace = useSpaceStore((state) => state.deleteSpace);

  const permissions = user?.permissions ?? [];
  const canReadSpaceSettings = permissions.includes('space.read');
  const canWriteSpaceSettings = permissions.includes('space.write');
  const canReadSpaceMembers = canReadSpaceSettings && permissions.includes('account.read');
  const canWriteSpaceMembers = canWriteSpaceSettings && permissions.includes('account.write');
  const activeMemberSpaceId = selectedMemberSpaceId ?? spaceUsages[0]?.spaceId ?? null;

  const permissionLabel = useCallback((permission?: SpacePermission) => {
    switch (permission) {
      case 'read':
        return t('spaceSettings.memberPermissionRead');
      case 'write':
        return t('spaceSettings.memberPermissionWrite');
      case 'manage':
        return t('spaceSettings.memberPermissionManage');
      default:
        return t('spaceSettings.memberPermissionNone');
    }
  }, [t]);

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

  const loadMembers = useCallback(async (spaceId: number) => {
    setMemberLoading(true);
    try {
      const members = await listSpaceMembers(spaceId);
      const accounts = canWriteSpaceMembers ? await listAccounts() : [];
      const nextPermissionMap: Record<number, SpacePermission | undefined> = {};
      members.forEach((member) => {
        nextPermissionMap[member.userId] = member.permission;
      });
      setMemberAccounts(accounts);
      setSpaceMembers(members);
      setMemberPermissionMap(nextPermissionMap);
    } catch (error) {
      message.error(error instanceof Error ? error.message : t('spaceSettings.loadMembersFailed'));
    } finally {
      setMemberLoading(false);
    }
  }, [canWriteSpaceMembers, message, t]);

  useEffect(() => {
    if (!canReadSpaceSettings) {
      return;
    }
    void refreshSpaceSettings();
  }, [canReadSpaceSettings, refreshSpaceSettings]);

  useEffect(() => {
    if (!canReadSpaceMembers) {
      setSelectedMemberSpaceId(null);
      setMemberAccounts([]);
      setSpaceMembers([]);
      setMemberPermissionMap({});
      return;
    }
    if (spaceUsages.length === 0) {
      setSelectedMemberSpaceId(null);
      setMemberAccounts([]);
      setSpaceMembers([]);
      setMemberPermissionMap({});
      return;
    }
    setSelectedMemberSpaceId((current) => (
      current != null && spaceUsages.some((item) => item.spaceId === current)
        ? current
        : spaceUsages[0]?.spaceId ?? null
    ));
  }, [canReadSpaceMembers, spaceUsages]);

  useEffect(() => {
    if (!canReadSpaceMembers || activeMemberSpaceId == null) {
      return;
    }
    void loadMembers(activeMemberSpaceId);
  }, [activeMemberSpaceId, canReadSpaceMembers, loadMembers]);

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

  const handleMemberPermissionChange = (userId: number, value?: SpacePermission) => {
    setMemberPermissionMap((prev) => ({
      ...prev,
      [userId]: value,
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

  const handleSaveMembers = async () => {
    if (activeMemberSpaceId == null) {
      return;
    }
    if (!canWriteSpaceMembers) {
      message.error(t('spaceSettings.noMemberWritePermission'));
      return;
    }

    setMemberSaving(true);
    try {
      const payload: UserSpacePermission[] = Object.entries(memberPermissionMap)
        .filter(([, permission]) => Boolean(permission))
        .map(([userId, permission]) => ({
          userId: Number(userId),
          spaceId: activeMemberSpaceId,
          permission: permission as SpacePermission,
        }));

      await updateSpaceMembers(activeMemberSpaceId, payload);
      await Promise.all([
        loadMembers(activeMemberSpaceId),
        fetchSpaces(),
      ]);
      message.success(t('spaceSettings.saveMembersSuccess'));
    } catch (error) {
      message.error(error instanceof Error ? error.message : t('spaceSettings.saveMembersFailed'));
    } finally {
      setMemberSaving(false);
    }
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

  const memberRows: SpaceMemberRow[] = canWriteSpaceMembers
    ? memberAccounts.map((member) => ({
      id: member.id,
      username: member.username,
      nickname: member.nickname,
      role: member.role,
      assignedPermission: memberPermissionMap[member.id],
    }))
    : spaceMembers.map((member) => ({
      id: member.userId,
      username: member.username,
      nickname: member.nickname,
      role: member.role,
      assignedPermission: member.permission,
    }));

  const memberColumns = [
    {
      title: t('spaceSettings.memberColumnUsername'),
      dataIndex: 'username',
      key: 'username',
      render: (value: string) => <Text code>{value}</Text>,
    },
    {
      title: t('spaceSettings.memberColumnNickname'),
      dataIndex: 'nickname',
      key: 'nickname',
    },
    {
      title: t('spaceSettings.memberColumnRole'),
      dataIndex: 'role',
      key: 'role',
      render: (value: string) => <Tag>{value}</Tag>,
    },
    {
      title: t('spaceSettings.memberColumnPermission'),
      key: 'permission',
      render: (_: unknown, item: SpaceMemberRow) => (
        canWriteSpaceMembers ? (
          <Select<SpacePermission>
            size="small"
            value={memberPermissionMap[item.id]}
            allowClear
            placeholder={t('spaceSettings.memberPermissionNone')}
            style={{ minWidth: 140 }}
            options={[
              { value: 'read', label: permissionLabel('read') },
              { value: 'write', label: permissionLabel('write') },
              { value: 'manage', label: permissionLabel('manage') },
            ]}
            onChange={(value: SpacePermission | undefined) => handleMemberPermissionChange(item.id, value)}
          />
        ) : item.assignedPermission ? (
          <Tag color="processing">{permissionLabel(item.assignedPermission)}</Tag>
        ) : (
          <Text type="secondary">{permissionLabel(undefined)}</Text>
        )
      ),
    },
  ];

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

      {canReadSpaceMembers ? (
        <Card
          size="small"
          extra={(
            <Space size={8} wrap>
              <Select<number>
                size="small"
                value={activeMemberSpaceId ?? undefined}
                placeholder={t('spaceSettings.memberSpacePlaceholder')}
                style={{ minWidth: 180 }}
                options={spaceUsages.map((item) => ({
                  value: item.spaceId,
                  label: item.spaceName,
                }))}
                onChange={(value: number | undefined) => setSelectedMemberSpaceId(value ?? null)}
              />
              <Button
                size="small"
                icon={<ReloadOutlined />}
                onClick={() => activeMemberSpaceId != null ? void loadMembers(activeMemberSpaceId) : undefined}
                loading={memberLoading}
                disabled={activeMemberSpaceId == null}
              >
                {t('spaceSettings.refreshMembers')}
              </Button>
              {canWriteSpaceMembers ? (
                <Button
                  size="small"
                  type="primary"
                  onClick={() => void handleSaveMembers()}
                  loading={memberSaving}
                  disabled={activeMemberSpaceId == null}
                >
                  {t('spaceSettings.saveMembersAction')}
                </Button>
              ) : null}
            </Space>
          )}
        >
          <Space orientation="vertical" size="middle" style={{ width: '100%' }}>
            <Text type="secondary">
              {canWriteSpaceMembers ? t('spaceSettings.memberDescription') : t('spaceSettings.memberReadOnlyDescription')}
            </Text>
            {activeMemberSpaceId == null ? (
              <Text type="secondary">{t('spaceSettings.memberEmptySpaces')}</Text>
            ) : (
              <Table<SpaceMemberRow>
                size="small"
                rowKey={(item: SpaceMemberRow) => item.id}
                loading={memberLoading}
                columns={memberColumns}
                dataSource={memberRows}
                pagination={false}
                locale={{ emptyText: t('spaceSettings.memberEmptyAccounts') }}
              />
            )}
          </Space>
        </Card>
      ) : null}
    </Space>
  );
};

export default SpaceSettings;
