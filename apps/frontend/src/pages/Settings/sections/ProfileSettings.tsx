import { Button, Card, Descriptions, Space, Tag, Typography } from 'antd';
import { LogoutOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router';
import { useAuth } from '@/features/auth/useAuth';
import SettingSectionHeader from '../components/SettingSectionHeader';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

const ProfileSettings = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { user, logout } = useAuth();

  const handleLogout = async () => {
    await logout();
    navigate('/login', { replace: true });
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

      <Space size="small" className="settings-stack-full">
        <Button danger type="primary" size="small" icon={<LogoutOutlined />} onClick={() => void handleLogout()}>
          {t('profileSettings.logout')}
        </Button>
      </Space>
    </Space>
  );
};

export default ProfileSettings;
