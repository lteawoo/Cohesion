import { ConfigProvider, Layout, Switch, theme } from "antd";
import { Outlet } from "react-router";
import MainSider from "./MainSider";
import { useState } from "react";
import { useSpaces } from "@/features/space/hooks/useSpaces";
import type { Space } from "@/features/space/types";

const { Header, Content } = Layout;

const PageLayout = ({ isDarkMode, onThemeChange }: { isDarkMode: boolean, onThemeChange: (checked: boolean) => void }) => {
  const { token } = theme.useToken();
  const { spaces, refetch } = useSpaces();
  const [selectedPath, setSelectedPath] = useState<string>('');
  const [selectedSpace, setSelectedSpace] = useState<Space | undefined>(undefined);

  const handlePathSelect = (path: string, space?: Space) => {
    setSelectedPath(path);
    if (space) {
      setSelectedSpace(space);
    }
  };

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
          <MainSider
            spaces={spaces}
            onSpaceCreated={refetch}
            onPathSelect={handlePathSelect}
          />

          <Content style={{ position: 'relative', display: 'flex', flexDirection: 'column' }}>
              <main style={{ flex: 1, overflow: 'hidden' }}>
                  <Outlet context={{ selectedPath, selectedSpace, onPathChange: setSelectedPath }} />
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