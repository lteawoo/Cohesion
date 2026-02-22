import DirectorySetupModal from "@/features/space/components/DirectorySetupModal";
import FolderTree from "@/features/browse/components/FolderTree";
import { PlusOutlined, CloseOutlined, DeleteOutlined } from "@ant-design/icons";
import { Button, Layout, theme, App } from "antd";
import type { Space } from "@/features/space/types";
import { useMemo, useState } from "react";
import type { Key } from "react";
import { useSpaceStore } from "@/stores/spaceStore";
import { useBrowseStore } from "@/stores/browseStore";
import { useAuth } from "@/features/auth/useAuth";
import SidePanelShell from "@/components/common/SidePanelShell";
import { useLocation, useNavigate } from "react-router";

const { Sider } = Layout;

interface MainSiderProps {
  onPathSelect?: (path: string, space?: Space) => void;
  onAfterSelect?: () => void;
  onClosePanel?: () => void;
  containerType?: "sider" | "panel";
}

export default function MainSider({ onPathSelect, onAfterSelect, onClosePanel, containerType = "sider" }: MainSiderProps) {
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

  const handleDeleteSpace = (space: Space) => {
    modal.confirm({
      title: 'Space 삭제',
      content: `"${space.space_name}" Space를 삭제하시겠습니까? 이 작업은 되돌릴 수 없습니다.`,
      okText: '삭제',
      cancelText: '취소',
      okButtonProps: { danger: true, loading: isDeleting },
      onOk: async () => {
        try {
          setIsDeleting(true);
          await deleteSpaceAction(space.id);
          message.success('Space가 삭제되었습니다.');
        } catch (error) {
          message.error(`Space 삭제 실패: ${error instanceof Error ? error.message : '알 수 없는 오류'}`);
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
        title="Spaces"
        leftAction={containerType === "panel" ? (
          <Button
            className="panel-close-btn"
            type="text"
            icon={<CloseOutlined />}
            size="small"
            onClick={onClosePanel}
            aria-label="탐색 닫기"
            title="탐색 닫기"
          />
        ) : null}
        rightAction={canWriteSpaces ? (
          <Button
            type="text"
            icon={<PlusOutlined />}
            size="small"
            onClick={() => setIsOpen(true)}
            aria-label="Space 추가"
            title="Space 추가"
          />
        ) : null}
        footer={(
          <Button
            type="text"
            icon={<DeleteOutlined />}
            className={`layout-sider-footer-action${isTrashMode ? " layout-sider-footer-action-active" : ""}`}
            onClick={handleOpenTrash}
            aria-label="휴지통"
            title="휴지통"
            block
          >
            휴지통
          </Button>
        )}
      >
        <FolderTree
          onSelect={handleSelect}
          onSpaceDelete={canWriteSpaces ? handleDeleteSpace : undefined}
          selectedKeys={treeSelectedKeys}
          isSearchMode={isSearchMode}
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
