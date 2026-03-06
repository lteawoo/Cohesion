import { App, Button, Card, Descriptions, Input, Modal, Space, Tag, Typography } from 'antd';
import { LogoutOutlined } from '@ant-design/icons';
import { useEffect, useMemo, useState } from 'react';
import type { ChangeEvent } from 'react';
import { useNavigate } from 'react-router';
import { useAuth } from '@/features/auth/useAuth';
import SettingSectionHeader from '../components/SettingSectionHeader';
import { useTranslation } from 'react-i18next';
import { updateProfile } from '@/api/auth';

const { Text } = Typography;

const ProfileSettings = () => {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const navigate = useNavigate();
  const { user, logout, refreshSession } = useAuth();
  const [nickname, setNickname] = useState(user?.nickname ?? '');
  const [isPasswordModalOpen, setIsPasswordModalOpen] = useState(false);
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [savingProfile, setSavingProfile] = useState(false);
  const [savingPassword, setSavingPassword] = useState(false);

  const canWriteProfile = user?.permissions?.includes('profile.write') ?? false;

  useEffect(() => {
    setNickname(user?.nickname ?? '');
  }, [user?.nickname]);

  const hasNicknameChanges = useMemo(
    () => nickname.trim() !== (user?.nickname ?? ''),
    [nickname, user?.nickname],
  );

  const handleLogout = async () => {
    await logout();
    navigate('/login', { replace: true });
  };

  const resetPasswordForm = () => {
    setCurrentPassword('');
    setNewPassword('');
    setConfirmPassword('');
  };

  const handleClosePasswordModal = () => {
    if (savingPassword) {
      return;
    }
    setIsPasswordModalOpen(false);
    resetPasswordForm();
  };

  const handleSaveProfile = async () => {
    if (!canWriteProfile) {
      return;
    }

    const trimmedNickname = nickname.trim();
    if (trimmedNickname.length === 0) {
      message.error(t('profileSettings.nicknameRequired'));
      return;
    }

    setSavingProfile(true);
    try {
      await updateProfile({ nickname: trimmedNickname });
      await refreshSession();
      message.success(t('profileSettings.saveSuccess'));
    } catch (error) {
      message.error(error instanceof Error ? error.message : t('profileSettings.saveFailed'));
    } finally {
      setSavingProfile(false);
    }
  };

  const handleChangePassword = async () => {
    if (!canWriteProfile) {
      return;
    }
    if (currentPassword.trim().length === 0) {
      message.error(t('profileSettings.currentPasswordRequired'));
      return;
    }
    if (newPassword.trim().length < 8) {
      message.error(t('profileSettings.passwordMinLength8'));
      return;
    }
    if (newPassword !== confirmPassword) {
      message.error(t('profileSettings.passwordMismatch'));
      return;
    }

    setSavingPassword(true);
    try {
      await updateProfile({
        currentPassword,
        newPassword,
      });
      await refreshSession();
      handleClosePasswordModal();
      message.success(t('profileSettings.passwordChangeSuccess'));
    } catch (error) {
      message.error(error instanceof Error ? error.message : t('profileSettings.saveFailed'));
    } finally {
      setSavingPassword(false);
    }
  };

  return (
    <Space vertical size="small" className="settings-section">
      <SettingSectionHeader title={t('profileSettings.sectionTitle')} subtitle={t('profileSettings.sectionSubtitle')} />

      <Card size="small">
        <Descriptions
          bordered
          column={1}
          size="small"
          items={[
            {
              key: 'username',
              label: t('profileSettings.usernameLabel'),
              children: <Text code>{user?.username ?? '-'}</Text>,
            },
            {
              key: 'nickname',
              label: t('profileSettings.nicknameLabel'),
              children: <Text>{user?.nickname ?? '-'}</Text>,
            },
            {
              key: 'role',
              label: t('profileSettings.roleLabel'),
              children: (
                <Tag color={user?.role === 'admin' ? 'gold' : 'default'}>
                  {user?.role === 'admin'
                    ? t('profileSettings.roleAdmin')
                    : user?.role === 'user'
                      ? t('profileSettings.roleUser')
                      : '-'}
                </Tag>
              ),
            },
          ]}
        />
      </Card>

      <Card size="small">
        <Space vertical size="middle" className="settings-stack-full">
          <Space vertical size="small" className="settings-stack-full">
            <Text strong>{t('profileSettings.nicknameLabel')}</Text>
            <Input
              value={nickname}
              onChange={(event: ChangeEvent<HTMLInputElement>) => setNickname(event.target.value)}
              placeholder={t('profileSettings.nicknamePlaceholder')}
              disabled={!canWriteProfile || savingProfile}
            />
          </Space>
          {!canWriteProfile && (
            <Text type="secondary">{t('profileSettings.readOnlyHint')}</Text>
          )}
        </Space>
      </Card>

      <Space size="small" className="settings-stack-full">
        {canWriteProfile && (
          <>
            <Button
              type="primary"
              size="small"
              onClick={() => void handleSaveProfile()}
              disabled={!hasNicknameChanges}
              loading={savingProfile}
            >
              {t('profileSettings.save')}
            </Button>
            <Button
              size="small"
              onClick={() => setIsPasswordModalOpen(true)}
              disabled={savingProfile || savingPassword}
            >
              {t('profileSettings.changePasswordAction')}
            </Button>
          </>
        )}
        <Button danger type="primary" size="small" icon={<LogoutOutlined />} onClick={() => void handleLogout()}>
          {t('profileSettings.logout')}
        </Button>
      </Space>

      <Modal
        title={t('profileSettings.passwordModalTitle')}
        open={isPasswordModalOpen}
        onCancel={handleClosePasswordModal}
        onOk={() => void handleChangePassword()}
        okText={t('profileSettings.changePasswordAction')}
        cancelText={t('profileSettings.cancel')}
        confirmLoading={savingPassword}
        destroyOnHidden
      >
        <Space vertical size="small" className="settings-stack-full">
          <Text strong>{t('profileSettings.currentPasswordLabel')}</Text>
          <Input.Password
            autoComplete="current-password"
            value={currentPassword}
            onChange={(event: ChangeEvent<HTMLInputElement>) => setCurrentPassword(event.target.value)}
            placeholder={t('profileSettings.currentPasswordPlaceholder')}
            disabled={savingPassword}
          />
          <Text strong>{t('profileSettings.newPasswordLabel')}</Text>
          <Input.Password
            autoComplete="new-password"
            value={newPassword}
            onChange={(event: ChangeEvent<HTMLInputElement>) => setNewPassword(event.target.value)}
            placeholder={t('profileSettings.newPasswordPlaceholder')}
            disabled={savingPassword}
          />
          <Text strong>{t('profileSettings.confirmPasswordLabel')}</Text>
          <Input.Password
            autoComplete="new-password"
            value={confirmPassword}
            onChange={(event: ChangeEvent<HTMLInputElement>) => setConfirmPassword(event.target.value)}
            placeholder={t('profileSettings.confirmPasswordPlaceholder')}
            disabled={savingPassword}
          />
        </Space>
      </Modal>
    </Space>
  );
};

export default ProfileSettings;
