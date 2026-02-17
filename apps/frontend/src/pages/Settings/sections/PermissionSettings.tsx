import { useCallback, useEffect, useMemo, useState, type ChangeEvent } from 'react';
import { App, Button, Card, Checkbox, Input, Popconfirm, Select, Space, Typography } from 'antd';
import { DeleteOutlined, PlusOutlined, SaveOutlined } from '@ant-design/icons';
import {
  createRole,
  deleteRole,
  listPermissionDefinitions,
  listRoles,
  updateRolePermissions,
  type PermissionItem,
  type RoleItem,
} from '@/api/roles';
import SettingSectionHeader from '../components/SettingSectionHeader';
import { useAuth } from '@/features/auth/useAuth';

const { Text } = Typography;

const PermissionSettings = () => {
  const { message } = App.useApp();
  const { user, refreshSession } = useAuth();
  const [roles, setRoles] = useState<RoleItem[]>([]);
  const [permissions, setPermissions] = useState<PermissionItem[]>([]);
  const [selectedRole, setSelectedRole] = useState<string>('');
  const [selectedPermissionKeys, setSelectedPermissionKeys] = useState<string[]>([]);
  const [newRoleName, setNewRoleName] = useState('');
  const [newRoleDescription, setNewRoleDescription] = useState('');
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [creating, setCreating] = useState(false);
  const [deleting, setDeleting] = useState(false);

  const selectedRoleInfo = useMemo(
    () => roles.find((role) => role.name === selectedRole),
    [roles, selectedRole]
  );

  const roleOptions = useMemo(
    () => roles.map((role) => ({
      value: role.name,
      label: role.name,
    })),
    [roles]
  );

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const [roleList, permissionList] = await Promise.all([
        listRoles(),
        listPermissionDefinitions(),
      ]);
      setRoles(roleList);
      setPermissions(permissionList);
      setSelectedRole((prev) => {
        if (prev && roleList.some((role) => role.name === prev)) {
          return prev;
        }
        return roleList[0]?.name ?? '';
      });
    } catch (error) {
      message.error(error instanceof Error ? error.message : '권한 정보를 불러오지 못했습니다');
    } finally {
      setLoading(false);
    }
  }, [message]);

  useEffect(() => {
    void loadData();
  }, [loadData]);

  useEffect(() => {
    if (!selectedRoleInfo) {
      setSelectedPermissionKeys([]);
      return;
    }
    setSelectedPermissionKeys(selectedRoleInfo.permissions);
  }, [selectedRoleInfo]);

  const handleCreateRole = async () => {
    if (newRoleName.trim().length < 2) {
      message.error('Role 이름은 2자 이상이어야 합니다');
      return;
    }

    setCreating(true);
    try {
      const created = await createRole(newRoleName.trim(), newRoleDescription.trim());
      setRoles((prev) => [...prev, { ...created, permissions: [] }].sort((a, b) => a.name.localeCompare(b.name)));
      setSelectedRole(created.name);
      setNewRoleName('');
      setNewRoleDescription('');
      message.success('Role이 생성되었습니다');
    } catch (error) {
      message.error(error instanceof Error ? error.message : 'Role 생성에 실패했습니다');
    } finally {
      setCreating(false);
    }
  };

  const handleDeleteRole = async () => {
    if (!selectedRoleInfo) {
      return;
    }
    setDeleting(true);
    try {
      await deleteRole(selectedRoleInfo.name);
      const nextRoles = roles.filter((role) => role.name !== selectedRoleInfo.name);
      setRoles(nextRoles);
      setSelectedRole(nextRoles[0]?.name ?? '');
      message.success('Role이 삭제되었습니다');
    } catch (error) {
      message.error(error instanceof Error ? error.message : 'Role 삭제에 실패했습니다');
    } finally {
      setDeleting(false);
    }
  };

  const handleSavePermissions = async () => {
    if (!selectedRoleInfo) {
      return;
    }
    setSaving(true);
    try {
      await updateRolePermissions(selectedRoleInfo.name, selectedPermissionKeys);
      setRoles((prev) => prev.map((role) => (
        role.name === selectedRoleInfo.name
          ? { ...role, permissions: selectedPermissionKeys }
          : role
      )));
      if (user?.role === selectedRoleInfo.name) {
        await refreshSession();
      }
      message.success('권한이 저장되었습니다');
    } catch (error) {
      message.error(error instanceof Error ? error.message : '권한 저장에 실패했습니다');
    } finally {
      setSaving(false);
    }
  };

  return (
    <Space vertical size="small" className="settings-section">
      <SettingSectionHeader title="권한 관리" subtitle="Role 생성/삭제 및 Role별 권한을 관리합니다" />

      <Card size="small" loading={loading}>
        <Space vertical size="small" className="settings-stack-full">
          <Space size="small" wrap>
            <Select
              value={selectedRole || undefined}
              options={roleOptions}
              onChange={(value: string) => setSelectedRole(value)}
              style={{ minWidth: 180 }}
              placeholder="Role 선택"
            />
            <Button
              type="primary"
              icon={<SaveOutlined />}
              onClick={() => void handleSavePermissions()}
              loading={saving}
              disabled={!selectedRole}
            >
              권한 저장
            </Button>
            <Popconfirm
              title="Role 삭제"
              description={`${selectedRole || ''} Role을 삭제하시겠습니까?`}
              okText="삭제"
              cancelText="취소"
              onConfirm={() => void handleDeleteRole()}
              disabled={!selectedRoleInfo || selectedRoleInfo.isSystem}
            >
              <Button
                danger
                icon={<DeleteOutlined />}
                disabled={!selectedRoleInfo || selectedRoleInfo.isSystem}
                loading={deleting}
              >
                Role 삭제
              </Button>
            </Popconfirm>
          </Space>

          {selectedRoleInfo && (
            <Space vertical size={4}>
              <Text strong>선택 Role: {selectedRoleInfo.name}</Text>
              <Text type="secondary">
                {selectedRoleInfo.isSystem ? '시스템 기본 Role(삭제 불가)' : (selectedRoleInfo.description || '설명 없음')}
              </Text>
            </Space>
          )}

          <Checkbox.Group
            value={selectedPermissionKeys}
            onChange={(values) => setSelectedPermissionKeys(values as string[])}
            style={{ width: '100%' }}
          >
            <Space vertical size={8} className="settings-stack-full">
              {permissions.map((permission) => (
                <Checkbox key={permission.key} value={permission.key}>
                  <Space size={6}>
                    <Text code>{permission.key}</Text>
                    <Text type="secondary">{permission.description}</Text>
                  </Space>
                </Checkbox>
              ))}
            </Space>
          </Checkbox.Group>
        </Space>
      </Card>

      <Card size="small" title="새 Role 추가">
        <Space vertical size="small" className="settings-stack-full">
          <Input
            placeholder="Role 이름 (예: manager)"
            value={newRoleName}
            onChange={(event: ChangeEvent<HTMLInputElement>) => setNewRoleName(event.target.value)}
          />
          <Input
            placeholder="설명 (선택)"
            value={newRoleDescription}
            onChange={(event: ChangeEvent<HTMLInputElement>) => setNewRoleDescription(event.target.value)}
          />
          <Button
            type="dashed"
            icon={<PlusOutlined />}
            onClick={() => void handleCreateRole()}
            loading={creating}
          >
            Role 추가
          </Button>
        </Space>
      </Card>
    </Space>
  );
};

export default PermissionSettings;
