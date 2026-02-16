import { Button, Card, Descriptions, Space, Tag, Typography } from 'antd';
import { LogoutOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router';
import { useAuth } from '@/features/auth/useAuth';
import SettingSectionHeader from '../components/SettingSectionHeader';

const { Text } = Typography;

const ProfileSettings = () => {
  const navigate = useNavigate();
  const { user, logout } = useAuth();

  const handleLogout = async () => {
    await logout();
    navigate('/login', { replace: true });
  };

  return (
    <Space vertical size="small" className="settings-section">
      <SettingSectionHeader title="내 프로필" subtitle="현재 로그인 계정 정보와 세션 관리" />

      <Card size="small">
        <Descriptions
          bordered
          column={1}
          size="small"
          items={[
            {
              key: 'username',
              label: '아이디',
              children: <Text code>{user?.username ?? '-'}</Text>,
            },
            {
              key: 'nickname',
              label: '닉네임',
              children: <Text>{user?.nickname ?? '-'}</Text>,
            },
            {
              key: 'role',
              label: '권한',
              children: (
                <Tag color={user?.role === 'admin' ? 'gold' : 'default'}>
                  {user?.role === 'admin' ? '관리자' : user?.role === 'user' ? '사용자' : '-'}
                </Tag>
              ),
            },
          ]}
        />
      </Card>

      <Space size="small" className="settings-stack-full">
        <Button danger type="primary" size="small" icon={<LogoutOutlined />} onClick={() => void handleLogout()}>
          로그아웃
        </Button>
      </Space>
    </Space>
  );
};

export default ProfileSettings;
