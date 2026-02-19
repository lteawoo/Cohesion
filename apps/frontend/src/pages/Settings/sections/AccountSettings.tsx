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
      message.error(error instanceof Error ? error.message : '계정 목록을 불러오지 못했습니다');
    } finally {
      setLoading(false);
    }
  }, [message]);

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
      message.error(error instanceof Error ? error.message : 'Role 목록을 불러오지 못했습니다');
    }
  }, [message]);

  useEffect(() => {
    void loadRoles();
  }, [loadRoles]);

  const validateCreate = (): boolean => {
    if (createForm.username.trim().length < 3) {
      message.error('아이디는 3자 이상이어야 합니다');
      return false;
    }
    if (createForm.password.trim().length < 6) {
      message.error('비밀번호는 6자 이상이어야 합니다');
      return false;
    }
    if (createForm.nickname.trim().length === 0) {
      message.error('닉네임을 입력하세요');
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
      message.success('계정이 생성되었습니다');
      setIsCreateModalOpen(false);
      setCreateForm(defaultCreateForm);
      await loadUsers();
    } catch (error) {
      message.error(error instanceof Error ? error.message : '계정을 생성하지 못했습니다');
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
      message.error('닉네임을 입력하세요');
      return false;
    }
    if (editForm.password.trim().length > 0 && editForm.password.trim().length < 6) {
      message.error('비밀번호는 6자 이상이어야 합니다');
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
      message.success('계정이 수정되었습니다');
      setEditTarget(null);
      await loadUsers();
    } catch (error) {
      message.error(error instanceof Error ? error.message : '계정을 수정하지 못했습니다');
    } finally {
      setUpdatingId(null);
    }
  };

  const handleDelete = async (user: AccountUser) => {
    setDeletingId(user.id);
    try {
      await deleteAccount(user.id);
      message.success('계정이 삭제되었습니다');
      await loadUsers();
    } catch (error) {
      message.error(error instanceof Error ? error.message : '계정을 삭제하지 못했습니다');
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
      message.error(error instanceof Error ? error.message : 'Space 권한을 불러오지 못했습니다');
    } finally {
      setPermissionLoading(false);
    }
  }, [message]);

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
      message.success('Space 권한이 저장되었습니다');
      setPermissionTarget(null);
      setSpacePermissionMap({});
      setSpaceList([]);
    } catch (error) {
      message.error(error instanceof Error ? error.message : 'Space 권한 저장에 실패했습니다');
    } finally {
      setPermissionSaving(false);
    }
  };

  const columns: ColumnsType<AccountUser> = [
    {
      title: '아이디',
      dataIndex: 'username',
      key: 'username',
      width: isMobile ? 120 : 180,
      ellipsis: true,
      render: (value: string) => <Text code>{value}</Text>,
    },
    {
      title: '닉네임',
      dataIndex: 'nickname',
      key: 'nickname',
      width: isMobile ? 120 : 180,
      ellipsis: true,
    },
    {
      title: 'Role',
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
      title: '작업',
      key: 'actions',
      width: isMobile ? 220 : 280,
      render: (_: unknown, record: AccountUser) => (
        <Space size="small" wrap>
          <Button
            size="small"
            onClick={() => void loadAccountPermissions(record)}
          >
            Space 권한
          </Button>
          <Button
            size="small"
            icon={<EditOutlined />}
            onClick={() => handleOpenEdit(record)}
          >
            수정
          </Button>
          <Popconfirm
            title="계정 삭제"
            description={`${record.username} 계정을 삭제하시겠습니까?`}
            okText="삭제"
            cancelText="취소"
            onConfirm={() => void handleDelete(record)}
          >
            <Button
              size="small"
              danger
              icon={<DeleteOutlined />}
              loading={deletingId === record.id}
            >
              삭제
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <Space vertical size="small" className="settings-section">
      <SettingSectionHeader title="계정 관리" subtitle="서비스 접근 계정을 관리합니다" />

      <Space size="small">
        <Button icon={<ReloadOutlined />} onClick={() => void loadUsers()} size="small">
          새로고침
        </Button>
        <Button
          type="primary"
          icon={<PlusOutlined />}
          size="small"
          onClick={() => setIsCreateModalOpen(true)}
        >
          계정 추가
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
          locale={{ emptyText: '등록된 계정이 없습니다' }}
        />
      </Card>

      <Modal
        title="계정 추가"
        open={isCreateModalOpen}
        onCancel={() => {
          setIsCreateModalOpen(false);
          setCreateForm(defaultCreateForm);
        }}
        onOk={() => void handleCreate()}
        okText="생성"
        cancelText="취소"
        okButtonProps={{ loading: creating }}
      >
        <Space orientation="vertical" size="small" style={{ width: '100%' }}>
          <Text strong>아이디</Text>
          <Input
            prefix={<UserOutlined />}
            autoComplete="username"
            value={createForm.username}
            onChange={(event: ChangeEvent<HTMLInputElement>) =>
              setCreateForm((prev) => ({ ...prev, username: event.target.value }))
            }
          />
          <Text strong>비밀번호</Text>
          <Input.Password
            autoComplete="new-password"
            value={createForm.password}
            onChange={(event: ChangeEvent<HTMLInputElement>) =>
              setCreateForm((prev) => ({ ...prev, password: event.target.value }))
            }
          />
          <Text strong>닉네임</Text>
          <Input
            value={createForm.nickname}
            onChange={(event: ChangeEvent<HTMLInputElement>) =>
              setCreateForm((prev) => ({ ...prev, nickname: event.target.value }))
            }
          />
          <Text strong>Role</Text>
          <Select
            options={roleOptions}
            value={createForm.role}
            onChange={(value: AccountRole) => setCreateForm((prev) => ({ ...prev, role: value }))}
          />
        </Space>
      </Modal>

      <Modal
        title="계정 수정"
        open={Boolean(editTarget)}
        onCancel={() => {
          setEditTarget(null);
          setEditForm({ nickname: '', password: '', role: 'user' });
        }}
        onOk={() => void handleEdit()}
        okText="저장"
        cancelText="취소"
        okButtonProps={{ loading: updatingId !== null }}
      >
        <Space orientation="vertical" size="small" style={{ width: '100%' }}>
          <Text strong>아이디</Text>
          <Input value={editTarget?.username} disabled />
          <Text strong>닉네임</Text>
          <Input
            value={editForm.nickname}
            onChange={(event: ChangeEvent<HTMLInputElement>) =>
              setEditForm((prev) => ({ ...prev, nickname: event.target.value }))
            }
          />
          <Text strong>비밀번호 (선택)</Text>
          <Input.Password
            placeholder="변경 시에만 입력"
            autoComplete="new-password"
            value={editForm.password}
            onChange={(event: ChangeEvent<HTMLInputElement>) =>
              setEditForm((prev) => ({ ...prev, password: event.target.value }))
            }
          />
          <Text strong>Role</Text>
          <Select
            options={roleOptions}
            value={editForm.role}
            onChange={(value: AccountRole) => setEditForm((prev) => ({ ...prev, role: value }))}
          />
        </Space>
      </Modal>

      <Modal
        title={permissionTarget ? `${permissionTarget.username} Space 권한` : 'Space 권한'}
        open={Boolean(permissionTarget)}
        onCancel={() => {
          setPermissionTarget(null);
          setSpacePermissionMap({});
          setSpaceList([]);
        }}
        onOk={() => void handleSavePermissions()}
        okText="저장"
        cancelText="취소"
        confirmLoading={permissionSaving}
      >
        <Space orientation="vertical" size="small" className="settings-stack-full">
          {permissionLoading ? (
            <Text type="secondary">불러오는 중...</Text>
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
                    { value: 'none', label: '없음' },
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
