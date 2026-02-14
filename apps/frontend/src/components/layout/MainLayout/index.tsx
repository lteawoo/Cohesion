import { ConfigProvider, Layout, Switch, Button, theme, App, Drawer, Grid } from "antd";
import { Outlet, useNavigate } from "react-router";
import { SettingOutlined, MenuOutlined } from "@ant-design/icons";
import MainSider from "./MainSider";
import ServerStatus from "./ServerStatus";
import { useCallback, useEffect, useState } from "react";
import type { Space } from "@/features/space/types";
import { useSettingsStore } from "@/stores/settingsStore";
import { useSpaceStore } from "@/stores/spaceStore";
import { useBrowseStore } from "@/stores/browseStore";
import ContextMenu from "@/components/ContextMenu";

const { Header, Content } = Layout;

const PageLayout = ({ isDarkMode, onThemeChange }: { isDarkMode: boolean, onThemeChange: (checked: boolean) => void }) => {
  const { token } = theme.useToken();
  const screens = Grid.useBreakpoint();
  const isMobile = !screens.lg;
  const navigate = useNavigate();
  const [isNavOpen, setIsNavOpen] = useState(false);
  const fetchSpaces = useSpaceStore((state) => state.fetchSpaces);
  const spaces = useSpaceStore((state) => state.spaces);
  const setPath = useBrowseStore((state) => state.setPath);

  useEffect(() => {
    fetchSpaces();
  }, [fetchSpaces]);

  const handlePathSelect = useCallback((path: string, space?: Space) => {
    // Space가 명시되지 않으면 경로에서 자동으로 찾기 (가장 긴 매칭 선택)
    if (space) {
      setPath(path, space);
    } else {
      const matchingSpaces = spaces?.filter(s => path.startsWith(s.space_path)) || [];
      const matchedSpace = matchingSpaces.length > 0
        ? matchingSpaces.reduce((longest, current) =>
            current.space_path.length > longest.space_path.length ? current : longest
          )
        : undefined;
      setPath(path, matchedSpace);
    }
  }, [setPath, spaces]);

  const closeNavDrawer = useCallback(() => {
    setIsNavOpen(false);
  }, []);

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
            {isMobile && (
              <Button
                type="text"
                icon={<MenuOutlined />}
                onClick={() => setIsNavOpen(true)}
              />
            )}
            <div style={{ color: token.colorText, fontSize: '20px' }}>Cohesion</div>
            <ServerStatus />
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <Button
            type="text"
            icon={<SettingOutlined />}
            onClick={() => navigate('/settings')}
          >
            설정
          </Button>
          <Switch checked={isDarkMode} onChange={onThemeChange} checkedChildren="Dark" unCheckedChildren="Light" />
        </div>
      </Header>
      <Layout>
          {!isMobile && (
          <MainSider
            onPathSelect={handlePathSelect}
          />
          )}

          <Drawer
            title={null}
            placement="left"
            open={isNavOpen}
            onClose={closeNavDrawer}
            width={isMobile ? "88vw" : 360}
            styles={{ body: { padding: 0 } }}
            mask={isMobile}
          >
            <MainSider
              onPathSelect={handlePathSelect}
              onAfterSelect={closeNavDrawer}
              containerType="panel"
            />
          </Drawer>

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
    const currentTheme = useSettingsStore((state) => state.theme);
    const setTheme = useSettingsStore((state) => state.setTheme);

    const isDarkMode = currentTheme === 'dark';

    const handleThemeChange = (checked: boolean) => {
        setTheme(checked ? 'dark' : 'light');
    };

    const currentAlgorithm = isDarkMode ? theme.darkAlgorithm : theme.defaultAlgorithm;

    return (
        <ConfigProvider theme={{ algorithm: currentAlgorithm }}>
          <App>
            <PageLayout isDarkMode={isDarkMode} onThemeChange={handleThemeChange} />
            <ContextMenu />
          </App>
        </ConfigProvider>
    )
}
