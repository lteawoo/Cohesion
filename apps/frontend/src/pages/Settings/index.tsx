import { ConfigProvider, Layout, Menu, Typography, Button, theme, App } from 'antd';
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
import type React from 'react';
import GeneralSettings from './sections/GeneralSettings';
import AppearanceSettings from './sections/AppearanceSettings';
import FileSettings from './sections/FileSettings';
import ServerSettings from './sections/ServerSettings';
import AdvancedSettings from './sections/AdvancedSettings';

const { Sider, Content, Header } = Layout;
const { Title } = Typography;

type SettingsSection = 'general' | 'appearance' | 'files' | 'server' | 'advanced';

const SettingsPage = () => {
  const { token } = theme.useToken();
  const navigate = useNavigate();
  const [selectedSection, setSelectedSection] = useState<SettingsSection>('general');
  const uiVars = {
    '--ui-bg-container': token.colorBgContainer,
    '--ui-border': token.colorBorder,
  } as React.CSSProperties;

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
    <Layout className="ui-page-shell" style={uiVars}>
      <Header className="ui-header" style={{ justifyContent: 'flex-start', gap: 16 }}>
        <Button
          type="text"
          icon={<HomeFilled style={{ fontSize: '20px' }} />}
          onClick={() => navigate('/')}
        />
        <Title level={4} style={{ margin: 0 }}>
          설정
        </Title>
      </Header>

      <Layout className="ui-content-shell">
        <Sider
          width={300}
          className="ui-sider"
          style={{ borderRight: `1px solid ${token.colorBorder}` }}
        >
          <div className="ui-pad-sm">
            <Menu
              mode="inline"
              selectedKeys={[selectedSection]}
              items={menuItems}
              onClick={({ key }: { key: string }) => setSelectedSection(key as SettingsSection)}
              style={{ height: '100%', borderRight: 0, borderRadius: 8 }}
            />
          </div>
        </Sider>

        <Content
          className="ui-main-scroll ui-page-padding"
          style={{ background: token.colorBgLayout }}
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
