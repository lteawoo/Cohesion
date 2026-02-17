import { ConfigProvider, Layout, Button, theme, App, Drawer, Grid } from "antd";
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
import HeaderBrand from "@/components/common/HeaderBrand";
import HeaderGroup from "@/components/common/HeaderGroup";

const { Header, Content } = Layout;

const PageLayout = () => {
  const { token } = theme.useToken();
  const screens = Grid.useBreakpoint();
  const isMobile = !screens.lg;
  const navigate = useNavigate();
  const [isNavOpen, setIsNavOpen] = useState(false);
  const fetchSpaces = useSpaceStore((state) => state.fetchSpaces);
  const spaces = useSpaceStore((state) => state.spaces);
  const selectedSpace = useBrowseStore((state) => state.selectedSpace);
  const setPath = useBrowseStore((state) => state.setPath);
  const clearContent = useBrowseStore((state) => state.clearContent);

  useEffect(() => {
    fetchSpaces();
  }, [fetchSpaces]);

  const handlePathSelect = useCallback((path: string, space?: Space) => {
    if (space) {
      setPath(path, space);
      return;
    }
    if (selectedSpace) {
      setPath(path, selectedSpace);
    }
  }, [setPath, selectedSpace]);

  useEffect(() => {
    if (!selectedSpace) {
      return;
    }
    const isSelectedSpaceAllowed = spaces.some((space) => space.id === selectedSpace.id);
    if (!isSelectedSpaceAllowed) {
      clearContent();
    }
  }, [spaces, selectedSpace, clearContent]);

  const closeNavDrawer = useCallback(() => {
    setIsNavOpen(false);
  }, []);

  return (
    <Layout className="layout-page">
      <Header
        className="layout-header"
        style={{
          background: token.colorBgContainer
        }}
      >
        <HeaderGroup align="start">
            {isMobile && (
              <Button
                type="text"
                icon={<MenuOutlined />}
                onClick={() => setIsNavOpen(true)}
              />
            )}
            <HeaderBrand text="Cohesion" color={token.colorText} />
            <ServerStatus />
        </HeaderGroup>
        <HeaderGroup align="end">
          <Button
            type="text"
            icon={<SettingOutlined />}
            onClick={() => navigate('/settings')}
            aria-label="설정"
            title="설정"
          />
        </HeaderGroup>
      </Header>
      <Layout className="layout-body">
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
            size={isMobile ? "default" : "large"}
            closeIcon={null}
            styles={{ body: { padding: 0 }, header: { display: "none" } }}
            mask={isMobile}
          >
            <MainSider
              onPathSelect={handlePathSelect}
              onAfterSelect={closeNavDrawer}
              containerType="panel"
            />
          </Drawer>

          <Content className="layout-content">
              <main className="layout-content-scroll layout-content-scroll-hidden">
                  <Outlet />
              </main>
          </Content>
      </Layout>
    </Layout>
  );
}

export default function MainLayout() {
    const currentTheme = useSettingsStore((state) => state.theme);

    const isDarkMode = currentTheme === 'dark';

    const currentAlgorithm = isDarkMode ? theme.darkAlgorithm : theme.defaultAlgorithm;

    return (
        <ConfigProvider theme={{ algorithm: currentAlgorithm }}>
          <App>
            <PageLayout />
            <ContextMenu />
          </App>
        </ConfigProvider>
    )
}
