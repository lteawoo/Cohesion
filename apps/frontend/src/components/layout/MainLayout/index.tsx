import { ConfigProvider, Layout, Switch, theme } from "antd";
import { Outlet } from "react-router";
import { MailOutlined } from "@ant-design/icons";
import MainSider from "./MainSider";
import type { ItemType } from "antd/es/menu/interface";
import { useState } from "react";

const { Header, Content } = Layout;

const items: ItemType[] = [
    { key: '1', icon: <MailOutlined />, label: 'My folder' },
];

const PageLayout = ({ isDarkMode, onThemeChange }: { isDarkMode: boolean, onThemeChange: (checked: boolean) => void }) => {
  const { token } = theme.useToken();

  return (
    <Layout
      style={{
          display: 'flex',
          minHeight: '100vh',
          overflow: 'hidden'
      }}
    >
      <Header
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          background: token.colorBgContainer
        }}
      >
        <div style={{ color: token.colorText, fontSize: '20px' }}>
            Cohesion
        </div>
        <Switch checked={isDarkMode} onChange={onThemeChange} checkedChildren="Dark" unCheckedChildren="Light" />
      </Header>
      <Layout>
          <MainSider spaceItems={items} />

          <Content>
              <main style={{ flex: 1, overflowY: 'auto' }}>
                  <Outlet />
              </main>
          </Content>
      </Layout>
    </Layout>
  );
}

export default function MainLayout() {
    const [isDarkMode, setIsDarkMode] = useState(true);

    const handleThemeChange = (checked: boolean) => {
        setIsDarkMode(checked);
    };

    const currentAlgorithm = isDarkMode ? theme.darkAlgorithm : theme.defaultAlgorithm;

    return (
        <ConfigProvider theme={{ algorithm: currentAlgorithm }}>
          <PageLayout isDarkMode={isDarkMode} onThemeChange={handleThemeChange} />
        </ConfigProvider>
    )
}