import { ConfigProvider, Layout, Switch, theme } from "antd";
import { Outlet } from "react-router";
import MainSider from "./MainSider";
import ServerStatus from "./ServerStatus";
import { useCallback, useEffect } from "react";
import type { Space } from "@/features/space/types";
import { useThemeStore } from "@/stores/themeStore";
import { useSpaceStore } from "@/stores/spaceStore";
import { useBrowseStore } from "@/stores/browseStore";
import ContextMenu from "@/components/ContextMenu";

const { Header, Content } = Layout;

const PageLayout = ({ isDarkMode, onThemeChange }: { isDarkMode: boolean, onThemeChange: (checked: boolean) => void }) => {
  const { token } = theme.useToken();
  const fetchSpaces = useSpaceStore((state) => state.fetchSpaces);
  const spaces = useSpaceStore((state) => state.spaces);
  const setPath = useBrowseStore((state) => state.setPath);

  useEffect(() => {
    fetchSpaces();
  }, [fetchSpaces]);

  const handlePathSelect = useCallback((path: string, space?: Space) => {
    // Space가 명시되지 않으면 경로에서 자동으로 찾기
    if (space) {
      setPath(path, space);
    } else {
      const matchedSpace = spaces?.find(s => path.startsWith(s.space_path));
      setPath(path, matchedSpace);
    }
  }, [setPath, spaces]);

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
            onPathSelect={handlePathSelect}
          />

          <Content style={{ position: 'relative', display: 'flex', flexDirection: 'column' }}>
              <main style={{ flex: 1, overflow: 'hidden' }}>
                  <Outlet />
              </main>
          </Content>
      </Layout>
    </Layout>
  );
}

export default function MainLayout() {
    const isDarkMode = useThemeStore((state) => state.isDarkMode);
    const setTheme = useThemeStore((state) => state.setTheme);

    const handleThemeChange = (checked: boolean) => {
        setTheme(checked);
    };

    const currentAlgorithm = isDarkMode ? theme.darkAlgorithm : theme.defaultAlgorithm;

    return (
        <ConfigProvider theme={{ algorithm: currentAlgorithm }}>
          <PageLayout isDarkMode={isDarkMode} onThemeChange={handleThemeChange} />
          <ContextMenu />
        </ConfigProvider>
    )
}