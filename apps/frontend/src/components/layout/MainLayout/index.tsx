import { Layout, Button, theme, Drawer, Grid } from "antd";
import { Outlet, useNavigate } from "react-router";
import { SettingOutlined, MenuOutlined } from "@ant-design/icons";
import MainSider from "./MainSider";
import ServerStatus from "./ServerStatus";
import { useCallback, useEffect, useState } from "react";
import type { MouseEvent } from "react";
import type { Space } from "@/features/space/types";
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

  const handleContextMenuCapture = useCallback((event: MouseEvent<HTMLElement>) => {
    const target = event.target as HTMLElement | null;
    if (!target) {
      event.preventDefault();
      return;
    }

    const isEditableElement = Boolean(
      target.closest('input, textarea, [contenteditable="true"], [contenteditable=""], .allow-native-context-menu')
    );

    if (isEditableElement) {
      return;
    }

    // 탐색 앱 쉘에서는 브라우저 기본 우클릭 메뉴를 막고,
    // 허용된 위치에서는 각 컴포넌트의 커스텀 컨텍스트 메뉴를 사용합니다.
    event.preventDefault();
  }, []);

  return (
    <Layout className="layout-page layout-page-browse-shell" onContextMenuCapture={handleContextMenuCapture}>
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
            rootClassName="app-drawer app-drawer--no-header app-drawer--nav"
            title={null}
            placement="left"
            open={isNavOpen}
            onClose={closeNavDrawer}
            size={isMobile ? "default" : "large"}
            closeIcon={null}
            mask={isMobile}
            maskClosable
          >
            <MainSider
              onPathSelect={handlePathSelect}
              onAfterSelect={closeNavDrawer}
              onClosePanel={closeNavDrawer}
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
    return (
      <>
        <PageLayout />
        <ContextMenu />
      </>
    )
}
