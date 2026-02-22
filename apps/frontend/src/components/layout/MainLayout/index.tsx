import { Layout, Button, theme, Drawer, Grid, Input, Spin } from "antd";
import { Outlet, useLocation, useNavigate } from "react-router";
import { SettingOutlined, MenuOutlined, SearchOutlined, CloseOutlined, FileOutlined, FolderFilled } from "@ant-design/icons";
import MainSider from "./MainSider";
import ServerStatus from "./ServerStatus";
import { useCallback, useEffect, useRef, useState } from "react";
import type { ChangeEvent, MouseEvent } from "react";
import type { Space } from "@/features/space/types";
import { useSpaceStore } from "@/stores/spaceStore";
import { useBrowseStore } from "@/stores/browseStore";
import ContextMenu from "@/components/ContextMenu";
import HeaderBrand from "@/components/common/HeaderBrand";
import HeaderGroup from "@/components/common/HeaderGroup";
import { searchFiles } from "@/features/search/api/searchApi";
import type { SearchFileResult } from "@/features/search/types";

const { Header, Content } = Layout;
const HEADER_SEARCH_RESULT_LIMIT = 8;
const HEADER_SEARCH_MIN_QUERY_LENGTH = 2;

const PageLayout = () => {
  const { token } = theme.useToken();
  const screens = Grid.useBreakpoint();
  const isMobile = !screens.lg;
  const location = useLocation();
  const navigate = useNavigate();
  const [isNavOpen, setIsNavOpen] = useState(false);
  const [isMobileSearchOpen, setIsMobileSearchOpen] = useState(false);
  const [headerSearchQuery, setHeaderSearchQuery] = useState("");
  const [headerSearchResults, setHeaderSearchResults] = useState<SearchFileResult[]>([]);
  const [isHeaderSearchLoading, setIsHeaderSearchLoading] = useState(false);
  const [headerSearchError, setHeaderSearchError] = useState<string | null>(null);
  const headerSearchTimerRef = useRef<number | undefined>(undefined);
  const headerSearchRequestSeqRef = useRef(0);
  const fetchSpaces = useSpaceStore((state) => state.fetchSpaces);
  const spaces = useSpaceStore((state) => state.spaces);
  const selectedSpace = useBrowseStore((state) => state.selectedSpace);
  const setPath = useBrowseStore((state) => state.setPath);
  const clearContent = useBrowseStore((state) => state.clearContent);
  const hasConnectedSpaces = spaces.length > 0;
  const normalizedHeaderSearchQuery = headerSearchQuery.trim();
  const showHeaderSearchInput = !isMobile || isMobileSearchOpen;
  const showHeaderSearchDropdown =
    showHeaderSearchInput &&
    normalizedHeaderSearchQuery.length >= HEADER_SEARCH_MIN_QUERY_LENGTH &&
    (isHeaderSearchLoading || headerSearchError !== null || headerSearchResults.length > 0);

  useEffect(() => {
    fetchSpaces();
  }, [fetchSpaces]);

  useEffect(() => {
    document.body.classList.add('browse-shell-active');
    return () => {
      document.body.classList.remove('browse-shell-active');
    };
  }, []);

  const clearHeaderSearchTimer = useCallback(() => {
    if (headerSearchTimerRef.current !== undefined) {
      window.clearTimeout(headerSearchTimerRef.current);
      headerSearchTimerRef.current = undefined;
    }
  }, []);

  useEffect(() => {
    return () => {
      clearHeaderSearchTimer();
    };
  }, [clearHeaderSearchTimer]);

  const resetHeaderSearchState = useCallback(() => {
    setHeaderSearchResults([]);
    setIsHeaderSearchLoading(false);
    setHeaderSearchError(null);
  }, []);

  const runHeaderSearch = useCallback(async (query: string, requestSeq: number) => {
    try {
      const data = await searchFiles(query, HEADER_SEARCH_RESULT_LIMIT);
      if (requestSeq !== headerSearchRequestSeqRef.current) {
        return;
      }
      setHeaderSearchResults(data);
      setHeaderSearchError(null);
    } catch (error) {
      if (requestSeq !== headerSearchRequestSeqRef.current) {
        return;
      }
      setHeaderSearchResults([]);
      setHeaderSearchError(error instanceof Error ? error.message : "검색 결과를 불러오지 못했습니다.");
    } finally {
      if (requestSeq === headerSearchRequestSeqRef.current) {
        setIsHeaderSearchLoading(false);
      }
    }
  }, []);

  const handlePathSelect = useCallback((path: string, space?: Space) => {
    if (space) {
      setPath(path, space);
    } else if (selectedSpace) {
      setPath(path, selectedSpace);
    }

    if (location.pathname !== "/") {
      navigate("/");
    }
  }, [location.pathname, navigate, setPath, selectedSpace]);

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

  const handleMobileSearchClose = useCallback(() => {
    setIsMobileSearchOpen(false);
    clearHeaderSearchTimer();
    headerSearchRequestSeqRef.current += 1;
    resetHeaderSearchState();
  }, [clearHeaderSearchTimer, resetHeaderSearchState]);

  const handleMobileSearchToggle = useCallback(() => {
    if (isMobileSearchOpen) {
      handleMobileSearchClose();
      return;
    }
    setIsMobileSearchOpen(true);
  }, [handleMobileSearchClose, isMobileSearchOpen]);

  const handleHeaderSearchChange = useCallback((event: ChangeEvent<HTMLInputElement>) => {
    const nextValue = event.target.value;
    setHeaderSearchQuery(nextValue);
    clearHeaderSearchTimer();

    const normalized = nextValue.trim();
    if (!hasConnectedSpaces || normalized.length < HEADER_SEARCH_MIN_QUERY_LENGTH) {
      headerSearchRequestSeqRef.current += 1;
      resetHeaderSearchState();
      return;
    }

    const requestSeq = headerSearchRequestSeqRef.current + 1;
    headerSearchRequestSeqRef.current = requestSeq;
    setIsHeaderSearchLoading(true);
    setHeaderSearchError(null);
    headerSearchTimerRef.current = window.setTimeout(() => {
      void runHeaderSearch(normalized, requestSeq);
    }, 220);
  }, [clearHeaderSearchTimer, hasConnectedSpaces, resetHeaderSearchState, runHeaderSearch]);

  const handleHeaderSearchSubmit = useCallback((rawQuery?: string) => {
    const keyword = (rawQuery ?? headerSearchQuery).trim();
    if (!hasConnectedSpaces || keyword.length < HEADER_SEARCH_MIN_QUERY_LENGTH) {
      return;
    }
    clearHeaderSearchTimer();
    headerSearchRequestSeqRef.current += 1;
    resetHeaderSearchState();
    if (isMobile) {
      setIsMobileSearchOpen(false);
    }
    navigate(`/search?q=${encodeURIComponent(keyword)}`);
  }, [clearHeaderSearchTimer, hasConnectedSpaces, headerSearchQuery, isMobile, navigate, resetHeaderSearchState]);

  const handleHeaderSearchResultSelect = useCallback((item: SearchFileResult) => {
    const targetSpace = spaces.find((space) => space.id === item.spaceId);
    if (!targetSpace) {
      return;
    }

    clearHeaderSearchTimer();
    headerSearchRequestSeqRef.current += 1;
    resetHeaderSearchState();
    setHeaderSearchQuery(item.name);
    setPath(item.isDir ? item.path : item.parentPath, targetSpace);

    if (isMobile) {
      setIsMobileSearchOpen(false);
    }
    navigate("/");
  }, [clearHeaderSearchTimer, isMobile, navigate, resetHeaderSearchState, setPath, spaces]);

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
        <div className="layout-header-center">
          {showHeaderSearchInput && (
            <div className="layout-header-search-shell">
              <div className="layout-header-search-field">
                <Input
                  allowClear
                  autoFocus={isMobile}
                  className="layout-header-search-input allow-text-select allow-native-context-menu"
                  disabled={!hasConnectedSpaces}
                  value={headerSearchQuery}
                  onChange={handleHeaderSearchChange}
                  onPressEnter={() => handleHeaderSearchSubmit()}
                  placeholder="검색"
                  prefix={<SearchOutlined />}
                />
                {showHeaderSearchDropdown && (
                  <div className="layout-header-search-dropdown">
                    {isHeaderSearchLoading ? (
                      <div className="layout-header-search-status">
                        <Spin size="small" />
                        <span>검색 중...</span>
                      </div>
                    ) : headerSearchError ? (
                      <div className="layout-header-search-status layout-header-search-status-error">
                        {headerSearchError}
                      </div>
                    ) : headerSearchResults.length > 0 ? (
                      <div className="layout-header-search-result-list">
                        {headerSearchResults.map((item) => (
                          <button
                            type="button"
                            key={`${item.spaceId}:${item.path}`}
                            className="layout-header-search-result-item"
                            onMouseDown={(event) => event.preventDefault()}
                            onClick={() => handleHeaderSearchResultSelect(item)}
                          >
                            <span className="layout-header-search-result-icon">
                              {item.isDir ? (
                                <FolderFilled style={{ color: "var(--app-folder-icon-color, #415a77)" }} />
                              ) : (
                                <FileOutlined />
                              )}
                            </span>
                            <span className="layout-header-search-result-main">
                              <span className="layout-header-search-result-name">{item.name}</span>
                              <span className="layout-header-search-result-meta">
                                {item.spaceName} · {item.parentPath || "/"}
                              </span>
                            </span>
                          </button>
                        ))}
                      </div>
                    ) : (
                      <div className="layout-header-search-status">검색 결과가 없습니다.</div>
                    )}
                    <button
                      type="button"
                      className="layout-header-search-submit"
                      onMouseDown={(event) => event.preventDefault()}
                      onClick={() => handleHeaderSearchSubmit(normalizedHeaderSearchQuery)}
                    >
                      Enter로 전체 결과 보기
                    </button>
                  </div>
                )}
              </div>
              {isMobile && (
                <Button
                  type="text"
                  icon={<CloseOutlined />}
                  aria-label="검색 닫기"
                  title="검색 닫기"
                  onClick={handleMobileSearchClose}
                />
              )}
            </div>
          )}
        </div>
        <HeaderGroup align="end">
          {isMobile && !isMobileSearchOpen && (
            <Button
              type="text"
              icon={<SearchOutlined />}
              onClick={handleMobileSearchToggle}
              aria-label="검색"
              title="검색"
            />
          )}
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
