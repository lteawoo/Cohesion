import DirectorySetupModal from "@/features/space/components/DirectorySetupModal";
import FolderTree from "@/features/browse/components/FolderTree";
import { PlusOutlined, CloseOutlined } from "@ant-design/icons";
import { Button, Layout, theme, App, Tree } from "antd";
import type { Space } from "@/features/space/types";
import { useMemo, useState } from "react";
import type { Key } from "react";
import { useSpaceStore } from "@/stores/spaceStore";
import { useBrowseStore } from "@/stores/browseStore";
import { useAuth } from "@/features/auth/useAuth";
import SidePanelShell from "@/components/common/SidePanelShell";
import { useLocation, useNavigate } from "react-router";
import { useTranslation } from "react-i18next";

const { Sider } = Layout;

interface MainSiderProps {
  onPathSelect?: (path: string, space?: Space) => void;
  onAfterSelect?: () => void;
  onClosePanel?: () => void;
  containerType?: "sider" | "panel";
}

export default function MainSider({ onPathSelect, onAfterSelect, onClosePanel, containerType = "sider" }: MainSiderProps) {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  const { message, modal } = App.useApp();
  const location = useLocation();
  const navigate = useNavigate();
  const isSearchMode = location.pathname === "/search";
  const isTrashMode = location.pathname === "/trash";
  const { user } = useAuth();
  const canWriteSpaces = (user?.permissions ?? []).includes("space.write");
  const [isOpen, setIsOpen] = useState(false);
  const deleteSpaceAction = useSpaceStore((state) => state.deleteSpace);
  const selectedPath = useBrowseStore((state) => state.selectedPath);
  const selectedSpace = useBrowseStore((state) => state.selectedSpace);
  const [isDeleting, setIsDeleting] = useState(false);
  const treeSelectedKeys = useMemo<Key[]>(() => {
    if (isSearchMode || isTrashMode) {
      return [];
    }
    if (!selectedSpace) {
      return [];
    }
    if (!selectedPath) {
      return [`space-${selectedSpace.id}`];
    }
    return [`space-${selectedSpace.id}::${selectedPath}`];
  }, [isSearchMode, isTrashMode, selectedPath, selectedSpace]);
  const trashTreeSelectedKeys = useMemo<Key[]>(
    () => (isTrashMode ? ["trash-action"] : []),
    [isTrashMode]
  );

  const handleDeleteSpace = (space: Space) => {
    modal.confirm({
      title: t("mainSider.deleteSpaceTitle"),
      content: t("mainSider.deleteSpaceContent", { spaceName: space.space_name }),
      okText: t("mainSider.delete"),
      cancelText: t("mainSider.cancel"),
      okButtonProps: { danger: true, loading: isDeleting },
      onOk: async () => {
        try {
          setIsDeleting(true);
          await deleteSpaceAction(space.id);
          message.success(t("mainSider.deleteSuccess"));
        } catch (error) {
          message.error(
            t("mainSider.deleteFailed", {
              error: error instanceof Error ? error.message : t("mainSider.unknownError"),
            })
          );
        } finally {
          setIsDeleting(false);
        }
      },
    });
  };

  const handleSelect = (path: string, space?: Space) => {
    (onPathSelect || (() => {}))(path, space);
    onAfterSelect?.();
  };

  const handleOpenTrash = () => {
    if (location.pathname !== "/trash") {
      navigate("/trash");
    }
    onAfterSelect?.();
  };

  const panelContent = (
    <>
      <DirectorySetupModal
        isOpen={isOpen}
        onClose={() => setIsOpen(false)}
      />
      <SidePanelShell
        title={t("mainSider.spacesTitle")}
        leftAction={containerType === "panel" ? (
          <Button
            className="panel-close-btn"
            type="text"
            icon={<CloseOutlined />}
            size="small"
            onClick={onClosePanel}
            aria-label={t("mainSider.closeNavigation")}
            title={t("mainSider.closeNavigation")}
          />
        ) : null}
        rightAction={canWriteSpaces ? (
          <Button
            type="text"
            icon={<PlusOutlined />}
            size="small"
            onClick={() => setIsOpen(true)}
            aria-label={t("mainSider.addSpace")}
            title={t("mainSider.addSpace")}
          />
        ) : null}
      >
        <FolderTree
          onSelect={handleSelect}
          onSpaceDelete={canWriteSpaces ? handleDeleteSpace : undefined}
          selectedKeys={treeSelectedKeys}
          isSearchMode={isSearchMode}
        />
        <Tree.DirectoryTree
          className="folder-tree layout-sider-tree-action"
          selectable
          selectedKeys={trashTreeSelectedKeys}
          treeData={[
            {
              key: "trash-action",
              title: (
                <span className="layout-sider-tree-action-title">
                  <span
                    className="material-symbols-rounded layout-sider-tree-action-icon"
                    style={{ fontVariationSettings: '"FILL" 1, "wght" 500, "GRAD" 0, "opsz" 20' }}
                    aria-hidden="true"
                  >
                    delete
                  </span>
                  <span>{t("mainSider.trash")}</span>
                </span>
              ),
              isLeaf: true,
            },
          ]}
          onSelect={() => handleOpenTrash()}
          expandAction={false}
          showIcon={false}
        />
      </SidePanelShell>
    </>
  );

  if (containerType === "panel") {
    return (
      <div
        className="layout-sider layout-sider-panel"
        style={{
          height: "100%",
          width: "100%",
          background: token.colorBgContainer,
        }}
      >
        {panelContent}
      </div>
    );
  }

  return (
    <Sider
      className="layout-sider"
      width={300}
      style={{
        background: token.colorBgContainer,
      }}
    >
      {panelContent}
    </Sider>
  );
}
