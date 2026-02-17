import { ConfigProvider, Layout, Menu, Button, theme, App } from 'antd';
import {
  UserOutlined,
  AppstoreOutlined,
  BgColorsOutlined,
  FileOutlined,
  GlobalOutlined,
  SafetyCertificateOutlined,
  TeamOutlined,
  HomeFilled,
} from '@ant-design/icons';
import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router';
import { useSettingsStore } from '@/stores/settingsStore';
import { useAuth } from '@/features/auth/useAuth';
import GeneralSettings from './sections/GeneralSettings';
import AppearanceSettings from './sections/AppearanceSettings';
import FileSettings from './sections/FileSettings';
import ServerSettings from './sections/ServerSettings';
import AccountSettings from './sections/AccountSettings';
import PermissionSettings from './sections/PermissionSettings';
import ProfileSettings from './sections/ProfileSettings';
import HeaderBrand from '@/components/common/HeaderBrand';
import HeaderGroup from '@/components/common/HeaderGroup';
import '@/assets/css/settings.css';

const { Sider, Content, Header } = Layout;

type SettingsSection = 'profile' | 'general' | 'appearance' | 'files' | 'server' | 'permissions' | 'accounts';

const SettingsPage = () => {
  const { token } = theme.useToken();
  const navigate = useNavigate();
  const { user } = useAuth();
  const [selectedSection, setSelectedSection] = useState<SettingsSection>('profile');

  const permissions = user?.permissions ?? [];
  const canAccessServerSettings = permissions.includes('server.config.read') || permissions.includes('server.config.write');
  const canAccessAccountSettings = permissions.includes('account.read') || permissions.includes('account.write');

  const menuItems = useMemo(() => [
    {
      key: 'profile',
      icon: <UserOutlined />,
      label: '내 프로필',
    },
    {
      key: 'general',
      icon: <AppstoreOutlined />,
      label: '일반',
    },
    {
      key: 'appearance',
      icon: <BgColorsOutlined />,
      label: '외관',
    },
    {
      key: 'files',
      icon: <FileOutlined />,
      label: '파일',
    },
    ...(canAccessServerSettings ? [{
      key: 'server',
      icon: <GlobalOutlined />,
      label: '서버',
    }] : []),
    ...(canAccessAccountSettings ? [{
      key: 'permissions',
      icon: <SafetyCertificateOutlined />,
      label: '권한 관리',
    }] : []),
    ...(canAccessAccountSettings ? [{
      key: 'accounts',
      icon: <TeamOutlined />,
      label: '계정 관리',
    }] : []),
  ], [canAccessAccountSettings, canAccessServerSettings]);

  const effectiveSection: SettingsSection = (
    (selectedSection === 'server' && !canAccessServerSettings) ||
    (selectedSection === 'permissions' && !canAccessAccountSettings) ||
    (selectedSection === 'accounts' && !canAccessAccountSettings)
  )
    ? 'profile'
    : selectedSection;

  const renderContent = () => {
    switch (effectiveSection) {
      case 'profile':
        return <ProfileSettings />;
      case 'general':
        return <GeneralSettings />;
      case 'appearance':
        return <AppearanceSettings />;
      case 'files':
        return <FileSettings />;
      case 'server':
        return canAccessServerSettings ? <ServerSettings /> : <ProfileSettings />;
      case 'permissions':
        return canAccessAccountSettings ? <PermissionSettings /> : <ProfileSettings />;
      case 'accounts':
        return canAccessAccountSettings ? <AccountSettings /> : <ProfileSettings />;
      default:
        return <ProfileSettings />;
    }
  };

  return (
    <Layout className="layout-page">
      <Header
        className="layout-header"
        style={{
          background: token.colorBgContainer,
        }}
      >
        <HeaderGroup align="start">
          <Button
            type="text"
            icon={<HomeFilled className="settings-icon-lg" />}
            onClick={() => navigate('/')}
          />
          <HeaderBrand text="설정" color={token.colorText} />
        </HeaderGroup>
      </Header>

      <Layout className="layout-body">
        <Sider
          className="layout-sider"
          width={300}
          style={{
            background: token.colorBgContainer,
          }}
        >
          <Menu
            className="settings-nav-menu settings-nav-menu-full"
            mode="inline"
            selectedKeys={[effectiveSection]}
            items={menuItems}
            onClick={({ key }: { key: string }) => setSelectedSection(key as SettingsSection)}
          />
        </Sider>

        <Content
          className="layout-content-scroll settings-content"
          style={{
            background: token.colorBgLayout,
          }}
        >
          {renderContent()}
        </Content>
      </Layout>
    </Layout>
  );
};

const Settings = () => {
  const currentTheme = useSettingsStore((state) => state.theme);
  const isDarkMode = currentTheme === 'dark';
  const currentAlgorithm = isDarkMode ? theme.darkAlgorithm : theme.defaultAlgorithm;

  return (
    <ConfigProvider theme={{ algorithm: currentAlgorithm }}>
      <App>
        <SettingsPage />
      </App>
    </ConfigProvider>
  );
};

export default Settings;
