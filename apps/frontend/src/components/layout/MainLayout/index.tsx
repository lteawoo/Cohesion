import { ConfigProvider, Layout, Switch, theme } from "antd";
import { Outlet } from "react-router";
import MainSider from "./MainSider";
import ServerStatus from "./ServerStatus";
import { useState, useCallback } from "react";
import { useSpaces } from "@/features/space/hooks/useSpaces";
import type { Space } from "@/features/space/types";
import { ContextMenuProvider } from "@/contexts/ContextMenuContext";

const { Header, Content } = Layout;

const PageLayout = ({ isDarkMode, onThemeChange }: { isDarkMode: boolean, onThemeChange: (checked: boolean) => void }) => {
  const { token } = theme.useToken();
  const { spaces, refetch } = useSpaces();
  const [pathState, setPathState] = useState<{
    path: string;
    space?: Space;
  }>({ path: '' });

  const handlePathSelect = useCallback((path: string, space?: Space) => {
    // 단일 setState로 통합하여 한 번의 리렌더링만 발생
    const newSpace = space || spaces?.find(s => path.startsWith(s.space_path));
    setPathState({ path, space: newSpace });
  }, [spaces]);

  return (
    <Layout
      style={{
          display: 'flex',
          height: '100vh',
          overflow: 'hidden'
      }}
    >
      <Header
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          padding: '0 16px',
          background: token.colorBgContainer
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
            <div style={{ color: token.colorText, fontSize: '20px' }}>Cohesion</div>
            <ServerStatus />
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
                  <Outlet context={{
                    selectedPath: pathState.path,
                    selectedSpace: pathState.space,
                    onPathChange: (path: string) => setPathState({ path, space: pathState.space })
                  }} />
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
          <ContextMenuProvider>
            <PageLayout isDarkMode={isDarkMode} onThemeChange={handleThemeChange} />
          </ContextMenuProvider>
        </ConfigProvider>
    )
}