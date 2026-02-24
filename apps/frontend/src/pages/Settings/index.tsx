import { Layout, Menu, Button, theme, Grid, Drawer } from 'antd';
import {
  UserOutlined,
  AppstoreOutlined,
  BgColorsOutlined,
  InfoCircleOutlined,
  GlobalOutlined,
  ClusterOutlined,
  SafetyCertificateOutlined,
  TeamOutlined,
  MenuOutlined,
  CloseOutlined,
} from '@ant-design/icons';
import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router';
import { useAuth } from '@/features/auth/useAuth';
import GeneralSettings from './sections/GeneralSettings';
import AppearanceSettings from './sections/AppearanceSettings';
import ServerSettings from './sections/ServerSettings';
import SpaceSettings from './sections/SpaceSettings';
import AccountSettings from './sections/AccountSettings';
import PermissionSettings from './sections/PermissionSettings';
import ProfileSettings from './sections/ProfileSettings';
import AboutSettings from './sections/AboutSettings';
import HeaderBrand from '@/components/common/HeaderBrand';
import HeaderGroup from '@/components/common/HeaderGroup';
import SidePanelShell from '@/components/common/SidePanelShell';
import { useTranslation } from 'react-i18next';
import '@/assets/css/settings.css';

const { Sider, Content, Header } = Layout;

type SettingsSection = 'profile' | 'general' | 'appearance' | 'server' | 'spaces' | 'permissions' | 'accounts' | 'about';

const SettingsPage = () => {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  const screens = Grid.useBreakpoint();
  const isMobile = !screens.lg;
  const navigate = useNavigate();
  const { user } = useAuth();
  const [selectedSection, setSelectedSection] = useState<SettingsSection>('profile');
  const [isNavOpen, setIsNavOpen] = useState(false);

  const permissions = user?.permissions ?? [];
  const canAccessServerSettings = permissions.includes('server.config.read') || permissions.includes('server.config.write');
  const canAccessSpaceSettings = permissions.includes('space.read') || permissions.includes('space.write');
  const canAccessAccountSettings = permissions.includes('account.read') || permissions.includes('account.write');

  const menuItems = useMemo(() => [
    {
      key: 'profile',
      icon: <UserOutlined />,
      label: t('settingsPage.sections.profile'),
    },
    {
      key: 'general',
      icon: <AppstoreOutlined />,
      label: t('settingsPage.sections.general'),
    },
    {
      key: 'appearance',
      icon: <BgColorsOutlined />,
      label: t('settingsPage.sections.appearance'),
    },
    ...(canAccessServerSettings ? [{
      key: 'server',
      icon: <GlobalOutlined />,
      label: t('settingsPage.sections.server'),
    }] : []),
    ...(canAccessSpaceSettings ? [{
      key: 'spaces',
      icon: <ClusterOutlined />,
      label: t('settingsPage.sections.spaces'),
    }] : []),
    ...(canAccessAccountSettings ? [{
      key: 'permissions',
      icon: <SafetyCertificateOutlined />,
      label: t('settingsPage.sections.permissions'),
    }] : []),
    ...(canAccessAccountSettings ? [{
      key: 'accounts',
      icon: <TeamOutlined />,
      label: t('settingsPage.sections.accounts'),
    }] : []),
    {
      key: 'about',
      icon: <InfoCircleOutlined />,
      label: t('settingsPage.sections.about'),
    },
  ], [canAccessAccountSettings, canAccessServerSettings, canAccessSpaceSettings, t]);

  const effectiveSection: SettingsSection = (
    (selectedSection === 'server' && !canAccessServerSettings) ||
    (selectedSection === 'spaces' && !canAccessSpaceSettings) ||
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
      case 'server':
        return canAccessServerSettings ? <ServerSettings /> : <ProfileSettings />;
      case 'spaces':
        return canAccessSpaceSettings ? <SpaceSettings /> : <ProfileSettings />;
      case 'permissions':
        return canAccessAccountSettings ? <PermissionSettings /> : <ProfileSettings />;
      case 'accounts':
        return canAccessAccountSettings ? <AccountSettings /> : <ProfileSettings />;
      case 'about':
        return <AboutSettings />;
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
              aria-label={t('settingsPage.openMenu')}
              title={t('settingsPage.menuTitle')}
            />
          )}
          <HeaderBrand
            text="Cohesion"
            color={token.colorText}
            onClick={() => navigate('/')}
            ariaLabel={t('mainLayout.goHome')}
            title={t('mainLayout.goHome')}
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
            <SidePanelShell title={t('settingsPage.sideTitle')}>
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
              title={t('settingsPage.sideTitle')}
              leftAction={(
                <Button
                  className="panel-close-btn"
                  type="text"
                  icon={<CloseOutlined />}
                  size="small"
                  onClick={() => setIsNavOpen(false)}
                  aria-label={t('settingsPage.closeMenu')}
                  title={t('settingsPage.closeMenu')}
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

export default SettingsPage;
