import { ConfigProvider, Layout, Menu, Button, theme, App } from 'antd';
import {
  AppstoreOutlined,
  BgColorsOutlined,
  FileOutlined,
  GlobalOutlined,
  ToolOutlined,
  HomeFilled,
} from '@ant-design/icons';
import { useState } from 'react';
import { useNavigate } from 'react-router';
import { useSettingsStore } from '@/stores/settingsStore';
import GeneralSettings from './sections/GeneralSettings';
import AppearanceSettings from './sections/AppearanceSettings';
import FileSettings from './sections/FileSettings';
import ServerSettings from './sections/ServerSettings';
import AdvancedSettings from './sections/AdvancedSettings';
import HeaderBrand from '@/components/common/HeaderBrand';
import HeaderGroup from '@/components/common/HeaderGroup';
import '@/assets/css/settings.css';

const { Sider, Content, Header } = Layout;

type SettingsSection = 'general' | 'appearance' | 'files' | 'server' | 'advanced';

const SettingsPage = () => {
  const { token } = theme.useToken();
  const navigate = useNavigate();
  const [selectedSection, setSelectedSection] = useState<SettingsSection>('general');

  const menuItems = [
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
    {
      key: 'server',
      icon: <GlobalOutlined />,
      label: '서버',
    },
    {
      key: 'advanced',
      icon: <ToolOutlined />,
      label: '고급',
    },
  ];

  const renderContent = () => {
    switch (selectedSection) {
      case 'general':
        return <GeneralSettings />;
      case 'appearance':
        return <AppearanceSettings />;
      case 'files':
        return <FileSettings />;
      case 'server':
        return <ServerSettings />;
      case 'advanced':
        return <AdvancedSettings />;
      default:
        return <GeneralSettings />;
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
            selectedKeys={[selectedSection]}
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
