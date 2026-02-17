import { ConfigProvider, Layout, Menu, Button, theme, App, Grid, Drawer } from 'antd';
import {
  UserOutlined,
  AppstoreOutlined,
  BgColorsOutlined,
  FileOutlined,
  GlobalOutlined,
  SafetyCertificateOutlined,
  TeamOutlined,
  MenuOutlined,
  CloseOutlined,
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
import SidePanelShell from '@/components/common/SidePanelShell';
import '@/assets/css/settings.css';

const { Sider, Content, Header } = Layout;

type SettingsSection = 'profile' | 'general' | 'appearance' | 'files' | 'server' | 'permissions' | 'accounts';

const SettingsPage = () => {
  const { token } = theme.useToken();
  const screens = Grid.useBreakpoint();
  const isMobile = !screens.lg;
  const navigate = useNavigate();
  const { user } = useAuth();
  const [selectedSection, setSelectedSection] = useState<SettingsSection>('profile');
  const [isNavOpen, setIsNavOpen] = useState(false);

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
          {isMobile && (
            <Button
              type="text"
              icon={<MenuOutlined />}
              onClick={() => setIsNavOpen(true)}
              aria-label="설정 메뉴 열기"
              title="설정 메뉴"
            />
          )}
          <HeaderBrand
            text="Cohesion"
            color={token.colorText}
            onClick={() => navigate('/')}
            ariaLabel="메인으로 이동"
            title="메인으로 이동"
          />
        </HeaderGroup>
      </Header>

      <Layout className="layout-body">
        {!isMobile && (
          <Sider
            className="layout-sider"
            width={300}
            style={{
              background: token.colorBgContainer,
            }}
          >
            <SidePanelShell title="설정">
              <Menu
                className="settings-nav-menu settings-nav-menu-full"
                mode="inline"
                selectedKeys={[effectiveSection]}
                items={menuItems}
                onClick={({ key }: { key: string }) => setSelectedSection(key as SettingsSection)}
              />
            </SidePanelShell>
          </Sider>
        )}

        <Content
          className="layout-content-scroll settings-content"
          style={{
            background: token.colorBgLayout,
          }}
        >
          {renderContent()}
        </Content>

        <Drawer
          rootClassName="app-drawer app-drawer--no-header app-drawer--nav app-drawer--settings-nav"
          placement="left"
          open={isMobile && isNavOpen}
          onClose={() => setIsNavOpen(false)}
          maskClosable
        >
          <div
            className="layout-sider layout-sider-panel"
            style={{
              height: '100%',
              width: '100%',
              background: token.colorBgContainer,
            }}
          >
            <SidePanelShell
              title="설정"
              leftAction={(
                <Button
                  className="panel-close-btn"
                  type="text"
                  icon={<CloseOutlined />}
                  size="small"
                  onClick={() => setIsNavOpen(false)}
                  aria-label="설정 메뉴 닫기"
                  title="설정 메뉴 닫기"
                />
              )}
            >
              <Menu
                className="settings-nav-menu settings-nav-menu-full"
                mode="inline"
                selectedKeys={[effectiveSection]}
                items={menuItems}
                onClick={({ key }: { key: string }) => {
                  setSelectedSection(key as SettingsSection);
                  setIsNavOpen(false);
                }}
              />
            </SidePanelShell>
          </div>
        </Drawer>
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
