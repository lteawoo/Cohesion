import { Layout, Button, theme, Drawer, Grid, Input, Spin } from "antd";
import { Outlet, useLocation, useNavigate } from "react-router";
import { SettingOutlined, MenuOutlined, SearchOutlined, CloseOutlined, FolderFilled } from "@ant-design/icons";
import MainSider from "./MainSider";
import ServerStatus from "./ServerStatus";
import { useCallback, useEffect, useRef, useState, memo } from "react";
import type { ChangeEvent, MouseEvent } from "react";
import type { Space } from "@/features/space/types";
import { useSpaceStore } from "@/stores/spaceStore";
import { useBrowseStore } from "@/stores/browseStore";
import ContextMenu from "@/components/ContextMenu";
import HeaderBrand from "@/components/common/HeaderBrand";
import HeaderGroup from "@/components/common/HeaderGroup";
import { searchFiles } from "@/features/search/api/searchApi";
import type { SearchFileResult } from "@/features/search/types";
import { FileTypeIcon } from "@/features/browse/components/FileTypeIcon";
import { useTranslation } from "react-i18next";

const { Header, Content } = Layout;
const HEADER_SEARCH_RESULT_LIMIT = 8;
const HEADER_SEARCH_SUBMIT_MIN_QUERY_LENGTH = 2;
const HEADER_SEARCH_SUGGEST_MIN_QUERY_LENGTH = 2;
const HEADER_SEARCH_DEBOUNCE_MS = 420;

function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === "AbortError";
}

interface HeaderSearchProps {
  isMobile: boolean;
  isMobileSearchMode: boolean;
  onMobileSearchClose: () => void;
}

const HeaderSearch = memo(function HeaderSearch({
  isMobile,
  isMobileSearchMode,
  onMobileSearchClose,
}: HeaderSearchProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const spaces = useSpaceStore((state) => state.spaces);
  const setPath = useBrowseStore((state) => state.setPath);
  const hasConnectedSpaces = spaces.length > 0;

  const [headerSearchQuery, setHeaderSearchQuery] = useState("");
  const [headerSearchResults, setHeaderSearchResults] = useState<SearchFileResult[]>([]);
  const [isHeaderSearchLoading, setIsHeaderSearchLoading] = useState(false);
  const [headerSearchError, setHeaderSearchError] = useState<string | null>(null);
  const headerSearchTimerRef = useRef<number | undefined>(undefined);
  const headerSearchAbortControllerRef = useRef<AbortController | null>(null);
  const headerSearchRequestSeqRef = useRef(0);

  const normalizedHeaderSearchQuery = headerSearchQuery.trim();
  const showHeaderSearchDropdown =
    normalizedHeaderSearchQuery.length >= HEADER_SEARCH_SUGGEST_MIN_QUERY_LENGTH &&
    (isHeaderSearchLoading || headerSearchError !== null || headerSearchResults.length > 0);

  const clearHeaderSearchTimer = useCallback(() => {
    if (headerSearchTimerRef.current !== undefined) {
      window.clearTimeout(headerSearchTimerRef.current);
      headerSearchTimerRef.current = undefined;
    }
  }, []);

  const clearHeaderSearchInFlight = useCallback(() => {
    if (headerSearchAbortControllerRef.current) {
      headerSearchAbortControllerRef.current.abort();
      headerSearchAbortControllerRef.current = null;
    }
  }, []);

  const resetHeaderSearchState = useCallback(() => {
    setHeaderSearchResults([]);
    setIsHeaderSearchLoading(false);
    setHeaderSearchError(null);
  }, []);

  useEffect(() => {
    return () => {
      clearHeaderSearchTimer();
      clearHeaderSearchInFlight();
    };
  }, [clearHeaderSearchInFlight, clearHeaderSearchTimer]);

  const runHeaderSearch = useCallback(async (
    query: string,
    requestSeq: number,
    controller: AbortController
  ) => {
    try {
      const data = await searchFiles(query, HEADER_SEARCH_RESULT_LIMIT, {
        signal: controller.signal,
      });
      if (requestSeq !== headerSearchRequestSeqRef.current) {
        return;
      }
      setHeaderSearchResults(data);
      setHeaderSearchError(null);
    } catch (error) {
      if (isAbortError(error)) {
        return;
      }
      if (requestSeq !== headerSearchRequestSeqRef.current) {
        return;
      }
      setHeaderSearchResults([]);
      setHeaderSearchError(error instanceof Error ? error.message : t("mainLayout.searchLoadFailed"));
    } finally {
      if (headerSearchAbortControllerRef.current === controller) {
        headerSearchAbortControllerRef.current = null;
      }
      if (requestSeq === headerSearchRequestSeqRef.current && !controller.signal.aborted) {
        setIsHeaderSearchLoading(false);
      }
    }
  }, [t]);

  const handleHeaderSearchChange = useCallback((event: ChangeEvent<HTMLInputElement>) => {
    const nextValue = event.target.value;
    setHeaderSearchQuery(nextValue);
    clearHeaderSearchTimer();
    clearHeaderSearchInFlight();

    const normalized = nextValue.trim();
    if (!hasConnectedSpaces || normalized.length < HEADER_SEARCH_SUGGEST_MIN_QUERY_LENGTH) {
      headerSearchRequestSeqRef.current += 1;
      resetHeaderSearchState();
      return;
    }

    const requestSeq = headerSearchRequestSeqRef.current + 1;
    headerSearchRequestSeqRef.current = requestSeq;
    setIsHeaderSearchLoading(true);
    setHeaderSearchError(null);
    headerSearchTimerRef.current = window.setTimeout(() => {
      const controller = new AbortController();
      headerSearchAbortControllerRef.current = controller;
      void runHeaderSearch(normalized, requestSeq, controller);
    }, HEADER_SEARCH_DEBOUNCE_MS);
  }, [clearHeaderSearchInFlight, clearHeaderSearchTimer, hasConnectedSpaces, resetHeaderSearchState, runHeaderSearch]);

  const handleHeaderSearchSubmit = useCallback((rawQuery?: string) => {
    const keyword = (rawQuery ?? headerSearchQuery).trim();
    if (!hasConnectedSpaces || keyword.length < HEADER_SEARCH_SUBMIT_MIN_QUERY_LENGTH) {
      return;
    }
    clearHeaderSearchTimer();
    clearHeaderSearchInFlight();
    headerSearchRequestSeqRef.current += 1;
    resetHeaderSearchState();
    if (isMobile) {
      onMobileSearchClose();
    }
    navigate(`/search?q=${encodeURIComponent(keyword)}`);
  }, [
    clearHeaderSearchInFlight,
    clearHeaderSearchTimer,
    hasConnectedSpaces,
    headerSearchQuery,
    isMobile,
    navigate,
    onMobileSearchClose,
    resetHeaderSearchState,
  ]);

  const handleHeaderSearchResultSelect = useCallback((item: SearchFileResult) => {
    const targetSpace = spaces.find((space) => space.id === item.spaceId);
    if (!targetSpace) {
      return;
    }

    clearHeaderSearchTimer();
    clearHeaderSearchInFlight();
    headerSearchRequestSeqRef.current += 1;
    resetHeaderSearchState();
    setHeaderSearchQuery(item.name);
    setPath(item.isDir ? item.path : item.parentPath, targetSpace);
    const searchQuery = normalizedHeaderSearchQuery || item.name;

    if (isMobile) {
      onMobileSearchClose();
    }
    navigate("/", {
      state: {
        fromSearchQuery: searchQuery,
      },
    });
  }, [
    clearHeaderSearchInFlight,
    clearHeaderSearchTimer,
    isMobile,
    navigate,
    normalizedHeaderSearchQuery,
    onMobileSearchClose,
    resetHeaderSearchState,
    setPath,
    spaces,
  ]);

  return (
    <div className="layout-header-search-field">
      <Input
        allowClear
        autoFocus={isMobileSearchMode}
        className="layout-header-search-input allow-text-select allow-native-context-menu"
        disabled={!hasConnectedSpaces}
        value={headerSearchQuery}
        onChange={handleHeaderSearchChange}
        onPressEnter={() => handleHeaderSearchSubmit()}
        placeholder={t("mainLayout.searchPlaceholder")}
        prefix={<SearchOutlined />}
      />
      {showHeaderSearchDropdown && (
        <div className="layout-header-search-dropdown">
          {isHeaderSearchLoading ? (
            <div className="layout-header-search-status">
              <Spin size="small" />
              <span>{t("mainLayout.searchLoading")}</span>
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
                      <FolderFilled style={{ color: "var(--app-folder-icon-color, #415a77)", fontSize: 18 }} />
                    ) : (
                      <FileTypeIcon filename={item.name} size={18} />
                    )}
                  </span>
                  <span className="layout-header-search-result-main">
                    <span className="layout-header-search-result-name">{item.name}</span>
                    <span className="layout-header-search-result-meta">
                      {item.spaceName}
                    </span>
                  </span>
                </button>
              ))}
            </div>
          ) : (
            <div className="layout-header-search-status">{t("mainLayout.searchNoResults")}</div>
          )}
          <button
            type="button"
            className="layout-header-search-submit"
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => handleHeaderSearchSubmit(normalizedHeaderSearchQuery)}
          >
            {t("mainLayout.searchViewAll")}
          </button>
        </div>
      )}
    </div>
  );
});

const PageLayout = () => {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  const screens = Grid.useBreakpoint();
  const isMobile = !screens.lg;
  const location = useLocation();
  const navigate = useNavigate();
  const [isNavOpen, setIsNavOpen] = useState(false);
  const [isMobileSearchOpen, setIsMobileSearchOpen] = useState(false);
  const fetchSpaces = useSpaceStore((state) => state.fetchSpaces);
  const spaces = useSpaceStore((state) => state.spaces);
  const selectedSpace = useBrowseStore((state) => state.selectedSpace);
  const setPath = useBrowseStore((state) => state.setPath);
  const clearContent = useBrowseStore((state) => state.clearContent);
  const isMobileSearchMode = isMobile && isMobileSearchOpen;
  const showHeaderSearchInput = !isMobile || isMobileSearchMode;

  useEffect(() => {
    fetchSpaces();
  }, [fetchSpaces]);

  useEffect(() => {
    document.body.classList.add('browse-shell-active');
    return () => {
      document.body.classList.remove('browse-shell-active');
    };
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
  }, []);

  const handleMobileSearchToggle = useCallback(() => {
    if (isMobileSearchOpen) {
      handleMobileSearchClose();
      return;
    }
    setIsMobileSearchOpen(true);
  }, [handleMobileSearchClose, isMobileSearchOpen]);

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

  const headerSearchField = showHeaderSearchInput ? (
    <HeaderSearch
      isMobile={isMobile}
      isMobileSearchMode={isMobileSearchMode}
      onMobileSearchClose={handleMobileSearchClose}
    />
  ) : null;

  return (
    <Layout className="layout-page layout-page-browse-shell" onContextMenuCapture={handleContextMenuCapture}>
      <Header
        className={`layout-header${isMobileSearchMode ? " layout-header-mobile-search" : ""}`}
        style={{
          background: token.colorBgContainer
        }}
      >
        {isMobileSearchMode ? (
          <div className="layout-header-mobile-search-shell">
            <Button
              type="text"
              icon={<CloseOutlined />}
              aria-label={t("mainLayout.closeSearch")}
              title={t("mainLayout.closeSearch")}
              onClick={handleMobileSearchClose}
            />
            {headerSearchField}
          </div>
        ) : (
          <>
            <HeaderGroup align="start">
                {isMobile && (
                  <Button
                    type="text"
                    icon={<MenuOutlined />}
                    onClick={() => setIsNavOpen(true)}
                  />
                )}
                <HeaderBrand
                  text="Cohesion"
                  color={token.colorText}
                  onClick={() => navigate("/")}
                  ariaLabel={t("mainLayout.goHome")}
                  title={t("mainLayout.goHome")}
                />
                <ServerStatus />
            </HeaderGroup>
            <div className="layout-header-center">
              {showHeaderSearchInput && (
                <div className="layout-header-search-shell">
                  {headerSearchField}
                </div>
              )}
            </div>
            <HeaderGroup align="end">
              {isMobile && (
                <Button
                  type="text"
                  icon={<SearchOutlined />}
                  onClick={handleMobileSearchToggle}
                  aria-label={t("mainLayout.search")}
                  title={t("mainLayout.search")}
                />
              )}
              <Button
                type="text"
                icon={<SettingOutlined />}
                onClick={() => navigate('/settings')}
                aria-label={t("mainLayout.settings")}
                title={t("mainLayout.settings")}
              />
            </HeaderGroup>
          </>
        )}
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
