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
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

const PermissionSettings = () => {
  const { t } = useTranslation();
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
      message.error(error instanceof Error ? error.message : t('permissionSettings.loadFailed'));
    } finally {
      setLoading(false);
    }
  }, [message, t]);

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
      message.error(t('permissionSettings.roleNameMin'));
      return;
    }

    setCreating(true);
    try {
      const created = await createRole(newRoleName.trim(), newRoleDescription.trim());
      setRoles((prev) => [...prev, { ...created, permissions: [] }].sort((a, b) => a.name.localeCompare(b.name)));
      setSelectedRole(created.name);
      setNewRoleName('');
      setNewRoleDescription('');
      message.success(t('permissionSettings.roleCreateSuccess'));
    } catch (error) {
      message.error(error instanceof Error ? error.message : t('permissionSettings.roleCreateFailed'));
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
      message.success(t('permissionSettings.roleDeleteSuccess'));
    } catch (error) {
      message.error(error instanceof Error ? error.message : t('permissionSettings.roleDeleteFailed'));
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
      message.success(t('permissionSettings.savePermissionsSuccess'));
    } catch (error) {
      message.error(error instanceof Error ? error.message : t('permissionSettings.savePermissionsFailed'));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Space vertical size="small" className="settings-section">
      <SettingSectionHeader title={t('permissionSettings.sectionTitle')} subtitle={t('permissionSettings.sectionSubtitle')} />

      <Card size="small" loading={loading}>
        <Space vertical size="small" className="settings-stack-full">
          <Space size="small" wrap>
            <Select
              value={selectedRole || undefined}
              options={roleOptions}
              onChange={(value: string) => setSelectedRole(value)}
              style={{ minWidth: 180 }}
              placeholder={t('permissionSettings.selectRolePlaceholder')}
            />
            <Button
              type="primary"
              icon={<SaveOutlined />}
              onClick={() => void handleSavePermissions()}
              loading={saving}
              disabled={!selectedRole}
            >
              {t('permissionSettings.savePermissionsButton')}
            </Button>
            <Popconfirm
              title={t('permissionSettings.deleteRoleTitle')}
              description={t('permissionSettings.deleteRoleDescription', { role: selectedRole || '' })}
              okText={t('permissionSettings.delete')}
              cancelText={t('permissionSettings.cancel')}
              onConfirm={() => void handleDeleteRole()}
              disabled={!selectedRoleInfo || selectedRoleInfo.isSystem}
            >
              <Button
                danger
                icon={<DeleteOutlined />}
                disabled={!selectedRoleInfo || selectedRoleInfo.isSystem}
                loading={deleting}
              >
                {t('permissionSettings.deleteRoleButton')}
              </Button>
            </Popconfirm>
          </Space>

          {selectedRoleInfo && (
            <Space vertical size={4}>
              <Text strong>{t('permissionSettings.selectedRole', { role: selectedRoleInfo.name })}</Text>
              <Text type="secondary">
                {selectedRoleInfo.isSystem
                  ? t('permissionSettings.systemRoleDescription')
                  : (selectedRoleInfo.description || t('permissionSettings.noDescription'))}
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

      <Card size="small" title={t('permissionSettings.newRoleCardTitle')}>
        <Space vertical size="small" className="settings-stack-full">
          <Input
            placeholder={t('permissionSettings.newRoleNamePlaceholder')}
            value={newRoleName}
            onChange={(event: ChangeEvent<HTMLInputElement>) => setNewRoleName(event.target.value)}
          />
          <Input
            placeholder={t('permissionSettings.newRoleDescriptionPlaceholder')}
            value={newRoleDescription}
            onChange={(event: ChangeEvent<HTMLInputElement>) => setNewRoleDescription(event.target.value)}
          />
          <Button
            type="dashed"
            icon={<PlusOutlined />}
            onClick={() => void handleCreateRole()}
            loading={creating}
          >
            {t('permissionSettings.addRoleButton')}
          </Button>
        </Space>
      </Card>
    </Space>
  );
};

export default PermissionSettings;
