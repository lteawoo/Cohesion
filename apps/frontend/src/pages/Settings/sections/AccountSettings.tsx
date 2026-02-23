import { useCallback, useEffect, useState } from 'react';
import { App, Button, Card, Grid, Input, Modal, Popconfirm, Select, Space, Table, Tag, Typography } from 'antd';
import { DeleteOutlined, EditOutlined, PlusOutlined, ReloadOutlined, UserOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import type { ChangeEvent } from 'react';
import {
  type AccountRole,
  type AccountUser,
  type SpacePermission,
  type UserSpacePermission,
  createAccount,
  deleteAccount,
  listAccountPermissions,
  listAccounts,
  listSpaces,
  updateAccountPermissions,
  updateAccount,
} from '@/api/accounts';
import { listRoles } from '@/api/roles';
import SettingSectionHeader from '../components/SettingSectionHeader';
import type { Space as SpaceItem } from '@/features/space/types';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

interface CreateAccountForm {
  username: string;
  password: string;
  nickname: string;
  role: AccountRole;
}

interface EditAccountForm {
  nickname: string;
  password: string;
  role: AccountRole;
}

const defaultCreateForm: CreateAccountForm = {
  username: '',
  password: '',
  nickname: '',
  role: 'user',
};

const AccountSettings = () => {
  const { t } = useTranslation();
  const screens = Grid.useBreakpoint();
  const isMobile = !screens.lg;
  const { message } = App.useApp();
  const [users, setUsers] = useState<AccountUser[]>([]);
  const [loading, setLoading] = useState(false);
  const [creating, setCreating] = useState(false);
  const [updatingId, setUpdatingId] = useState<number | null>(null);
  const [deletingId, setDeletingId] = useState<number | null>(null);
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const [createForm, setCreateForm] = useState<CreateAccountForm>(defaultCreateForm);
  const [editTarget, setEditTarget] = useState<AccountUser | null>(null);
  const [editForm, setEditForm] = useState<EditAccountForm>({
    nickname: '',
    password: '',
    role: 'user',
  });
  const [permissionTarget, setPermissionTarget] = useState<AccountUser | null>(null);
  const [spaceList, setSpaceList] = useState<SpaceItem[]>([]);
  const [spacePermissionMap, setSpacePermissionMap] = useState<Record<number, SpacePermission | undefined>>({});
  const [permissionLoading, setPermissionLoading] = useState(false);
  const [permissionSaving, setPermissionSaving] = useState(false);
  const [roleOptions, setRoleOptions] = useState<{ value: AccountRole; label: string }[]>([
    { value: 'admin', label: 'admin' },
    { value: 'user', label: 'user' },
  ]);

  const roleLabelMap = useCallback((role: AccountRole) => {
    const found = roleOptions.find((item) => item.value === role);
    return found?.label ?? role;
  }, [roleOptions]);

  const loadUsers = useCallback(async () => {
    setLoading(true);
    try {
      const data = await listAccounts();
      setUsers(data);
    } catch (error) {
      message.error(error instanceof Error ? error.message : t('accountSettings.loadAccountsFailed'));
    } finally {
      setLoading(false);
    }
  }, [message, t]);

  useEffect(() => {
    void loadUsers();
  }, [loadUsers]);

  const loadRoles = useCallback(async () => {
    try {
      const roles = await listRoles();
      setRoleOptions(roles.map((role) => ({
        value: role.name,
        label: role.name,
      })));
    } catch (error) {
      message.error(error instanceof Error ? error.message : t('accountSettings.loadRolesFailed'));
    }
  }, [message, t]);

  useEffect(() => {
    void loadRoles();
  }, [loadRoles]);

  const validateCreate = (): boolean => {
    if (createForm.username.trim().length < 3) {
      message.error(t('accountSettings.usernameMinLength'));
      return false;
    }
    if (createForm.password.trim().length < 6) {
      message.error(t('accountSettings.passwordMinLength'));
      return false;
    }
    if (createForm.nickname.trim().length === 0) {
      message.error(t('accountSettings.nicknameRequired'));
      return false;
    }
    return true;
  };

  const handleCreate = async () => {
    if (!validateCreate()) {
      return;
    }

    setCreating(true);
    try {
      await createAccount({
        username: createForm.username.trim(),
        password: createForm.password,
        nickname: createForm.nickname.trim(),
        role: createForm.role,
      });
      message.success(t('accountSettings.createSuccess'));
      setIsCreateModalOpen(false);
      setCreateForm(defaultCreateForm);
      await loadUsers();
    } catch (error) {
      message.error(error instanceof Error ? error.message : t('accountSettings.createFailed'));
    } finally {
      setCreating(false);
    }
  };

  const handleOpenEdit = (user: AccountUser) => {
    setEditTarget(user);
    setEditForm({
      nickname: user.nickname,
      password: '',
      role: user.role,
    });
  };

  const validateEdit = (): boolean => {
    if (editForm.nickname.trim().length === 0) {
      message.error(t('accountSettings.nicknameRequired'));
      return false;
    }
    if (editForm.password.trim().length > 0 && editForm.password.trim().length < 6) {
      message.error(t('accountSettings.passwordMinLength'));
      return false;
    }
    return true;
  };

  const handleEdit = async () => {
    if (!editTarget || !validateEdit()) {
      return;
    }

    setUpdatingId(editTarget.id);
    try {
      await updateAccount(editTarget.id, {
        nickname: editForm.nickname.trim(),
        role: editForm.role,
        ...(editForm.password.trim().length > 0 ? { password: editForm.password } : {}),
      });
      message.success(t('accountSettings.editSuccess'));
      setEditTarget(null);
      await loadUsers();
    } catch (error) {
      message.error(error instanceof Error ? error.message : t('accountSettings.editFailed'));
    } finally {
      setUpdatingId(null);
    }
  };

  const handleDelete = async (user: AccountUser) => {
    setDeletingId(user.id);
    try {
      await deleteAccount(user.id);
      message.success(t('accountSettings.deleteSuccess'));
      await loadUsers();
    } catch (error) {
      message.error(error instanceof Error ? error.message : t('accountSettings.deleteFailed'));
    } finally {
      setDeletingId(null);
    }
  };

  const loadAccountPermissions = useCallback(async (user: AccountUser) => {
    setPermissionLoading(true);
    try {
      const [spaces, permissions] = await Promise.all([
        listSpaces(),
        listAccountPermissions(user.id),
      ]);

      const permissionMap: Record<number, SpacePermission | undefined> = {};
      permissions.forEach((item) => {
        // legacy manage 값은 UI에서 read+write(write)로 매핑
        permissionMap[item.spaceId] = item.permission === 'manage' ? 'write' : item.permission;
      });

      setSpaceList(spaces);
      setSpacePermissionMap(permissionMap);
      setPermissionTarget(user);
    } catch (error) {
      message.error(error instanceof Error ? error.message : t('accountSettings.loadSpacePermissionsFailed'));
    } finally {
      setPermissionLoading(false);
    }
  }, [message, t]);

  const handleSavePermissions = async () => {
    if (!permissionTarget) return;
    setPermissionSaving(true);
    try {
      const payload: UserSpacePermission[] = Object.entries(spacePermissionMap)
        .filter(([, permission]) => Boolean(permission))
        .map(([spaceId, permission]) => ({
          userId: permissionTarget.id,
          spaceId: Number(spaceId),
          permission: permission as SpacePermission,
        }));

      await updateAccountPermissions(permissionTarget.id, payload);
      message.success(t('accountSettings.saveSpacePermissionsSuccess'));
      setPermissionTarget(null);
      setSpacePermissionMap({});
      setSpaceList([]);
    } catch (error) {
      message.error(error instanceof Error ? error.message : t('accountSettings.saveSpacePermissionsFailed'));
    } finally {
      setPermissionSaving(false);
    }
  };

  const columns: ColumnsType<AccountUser> = [
    {
      title: t('accountSettings.columnUsername'),
      dataIndex: 'username',
      key: 'username',
      width: isMobile ? 120 : 180,
      ellipsis: true,
      render: (value: string) => <Text code>{value}</Text>,
    },
    {
      title: t('accountSettings.columnNickname'),
      dataIndex: 'nickname',
      key: 'nickname',
      width: isMobile ? 120 : 180,
      ellipsis: true,
    },
    {
      title: t('accountSettings.columnRole'),
      dataIndex: 'role',
      key: 'role',
      width: 110,
      render: (role: AccountRole) => (
        <Tag color={role === 'admin' ? 'gold' : 'default'}>
          {roleLabelMap(role)}
        </Tag>
      ),
    },
    {
      title: t('accountSettings.columnActions'),
      key: 'actions',
      width: isMobile ? 220 : 280,
      render: (_: unknown, record: AccountUser) => (
        <Space size="small" wrap>
          <Button
            size="small"
            onClick={() => void loadAccountPermissions(record)}
          >
            {t('accountSettings.spacePermissionsButton')}
          </Button>
          <Button
            size="small"
            icon={<EditOutlined />}
            onClick={() => handleOpenEdit(record)}
          >
            {t('accountSettings.edit')}
          </Button>
          <Popconfirm
            title={t('accountSettings.deleteAccountTitle')}
            description={t('accountSettings.deleteAccountDescription', { username: record.username })}
            okText={t('accountSettings.delete')}
            cancelText={t('accountSettings.cancel')}
            onConfirm={() => void handleDelete(record)}
          >
            <Button
              size="small"
              danger
              icon={<DeleteOutlined />}
              loading={deletingId === record.id}
            >
              {t('accountSettings.delete')}
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <Space vertical size="small" className="settings-section">
      <SettingSectionHeader title={t('accountSettings.sectionTitle')} subtitle={t('accountSettings.sectionSubtitle')} />

      <Space size="small">
        <Button icon={<ReloadOutlined />} onClick={() => void loadUsers()} size="small">
          {t('accountSettings.refresh')}
        </Button>
        <Button
          type="primary"
          icon={<PlusOutlined />}
          size="small"
          onClick={() => setIsCreateModalOpen(true)}
        >
          {t('accountSettings.addAccount')}
        </Button>
      </Space>

      <Card size="small">
        <Table<AccountUser>
          rowKey="id"
          size="small"
          loading={loading}
          columns={columns}
          dataSource={users}
          pagination={false}
          scroll={{ x: isMobile ? 580 : undefined }}
          locale={{ emptyText: t('accountSettings.emptyAccounts') }}
        />
      </Card>

      <Modal
        title={t('accountSettings.createModalTitle')}
        open={isCreateModalOpen}
        onCancel={() => {
          setIsCreateModalOpen(false);
          setCreateForm(defaultCreateForm);
        }}
        onOk={() => void handleCreate()}
        okText={t('accountSettings.create')}
        cancelText={t('accountSettings.cancel')}
        okButtonProps={{ loading: creating }}
      >
        <Space orientation="vertical" size="small" style={{ width: '100%' }}>
          <Text strong>{t('accountSettings.fieldUsername')}</Text>
          <Input
            prefix={<UserOutlined />}
            autoComplete="username"
            value={createForm.username}
            onChange={(event: ChangeEvent<HTMLInputElement>) =>
              setCreateForm((prev) => ({ ...prev, username: event.target.value }))
            }
          />
          <Text strong>{t('accountSettings.fieldPassword')}</Text>
          <Input.Password
            autoComplete="new-password"
            value={createForm.password}
            onChange={(event: ChangeEvent<HTMLInputElement>) =>
              setCreateForm((prev) => ({ ...prev, password: event.target.value }))
            }
          />
          <Text strong>{t('accountSettings.fieldNickname')}</Text>
          <Input
            value={createForm.nickname}
            onChange={(event: ChangeEvent<HTMLInputElement>) =>
              setCreateForm((prev) => ({ ...prev, nickname: event.target.value }))
            }
          />
          <Text strong>{t('accountSettings.fieldRole')}</Text>
          <Select
            options={roleOptions}
            value={createForm.role}
            onChange={(value: AccountRole) => setCreateForm((prev) => ({ ...prev, role: value }))}
          />
        </Space>
      </Modal>

      <Modal
        title={t('accountSettings.editModalTitle')}
        open={Boolean(editTarget)}
        onCancel={() => {
          setEditTarget(null);
          setEditForm({ nickname: '', password: '', role: 'user' });
        }}
        onOk={() => void handleEdit()}
        okText={t('accountSettings.save')}
        cancelText={t('accountSettings.cancel')}
        okButtonProps={{ loading: updatingId !== null }}
      >
        <Space orientation="vertical" size="small" style={{ width: '100%' }}>
          <Text strong>{t('accountSettings.fieldUsername')}</Text>
          <Input value={editTarget?.username} disabled />
          <Text strong>{t('accountSettings.fieldNickname')}</Text>
          <Input
            value={editForm.nickname}
            onChange={(event: ChangeEvent<HTMLInputElement>) =>
              setEditForm((prev) => ({ ...prev, nickname: event.target.value }))
            }
          />
          <Text strong>{t('accountSettings.fieldPasswordOptional')}</Text>
          <Input.Password
            placeholder={t('accountSettings.passwordOptionalPlaceholder')}
            autoComplete="new-password"
            value={editForm.password}
            onChange={(event: ChangeEvent<HTMLInputElement>) =>
              setEditForm((prev) => ({ ...prev, password: event.target.value }))
            }
          />
          <Text strong>{t('accountSettings.fieldRole')}</Text>
          <Select
            options={roleOptions}
            value={editForm.role}
            onChange={(value: AccountRole) => setEditForm((prev) => ({ ...prev, role: value }))}
          />
        </Space>
      </Modal>

      <Modal
        title={permissionTarget
          ? t('accountSettings.spacePermissionsModalTitleWithUser', { username: permissionTarget.username })
          : t('accountSettings.spacePermissionsModalTitle')}
        open={Boolean(permissionTarget)}
        onCancel={() => {
          setPermissionTarget(null);
          setSpacePermissionMap({});
          setSpaceList([]);
        }}
        onOk={() => void handleSavePermissions()}
        okText={t('accountSettings.save')}
        cancelText={t('accountSettings.cancel')}
        confirmLoading={permissionSaving}
      >
        <Space orientation="vertical" size="small" className="settings-stack-full">
          {permissionLoading ? (
            <Text type="secondary">{t('accountSettings.loading')}</Text>
          ) : (
            spaceList.map((space) => (
                <div key={space.id} style={{ display: 'flex', gap: 8, alignItems: 'center', justifyContent: 'space-between' }}>
                  <div style={{ minWidth: 0, flex: 1 }}>
                    <Text strong>{space.space_name}</Text>
                  </div>
                  <Select
                  style={{ width: 140 }}
                  value={spacePermissionMap[space.id] ?? 'none'}
                  options={[
                    { value: 'none', label: t('accountSettings.permissionNone') },
                    { value: 'read', label: 'read' },
                    { value: 'write', label: 'read + write' },
                  ]}
                  onChange={(value: string) => {
                    setSpacePermissionMap((prev) => ({
                      ...prev,
                      [space.id]: value === 'none' ? undefined : value as SpacePermission,
                    }));
                  }}
                />
              </div>
            ))
          )}
        </Space>
      </Modal>
    </Space>
  );
};

export default AccountSettings;
