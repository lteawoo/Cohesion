import DirectorySetupModal from "@/features/space/components/DirectorySetupModal";
import FolderTree from "@/features/browse/components/FolderTree";
import { PlusOutlined } from "@ant-design/icons";
import { Button, Layout, theme, App } from "antd";
import type { Space } from "@/features/space/types";
import { useState } from "react";
import { useSpaceStore } from "@/stores/spaceStore";

const { Sider } = Layout;

interface MainSiderProps {
  onPathSelect?: (path: string, space?: Space) => void;
  onAfterSelect?: () => void;
  containerType?: "sider" | "panel";
}

export default function MainSider({ onPathSelect, onAfterSelect, containerType = "sider" }: MainSiderProps) {
  const { token } = theme.useToken();
  const { message, modal } = App.useApp();
  const [isOpen, setIsOpen] = useState(false);
  const deleteSpaceAction = useSpaceStore((state) => state.deleteSpace);
  const [isDeleting, setIsDeleting] = useState(false);

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

  const panelContent = (
    <>
      <DirectorySetupModal
        isOpen={isOpen}
        onClose={() => setIsOpen(false)}
      />
      <div
        className="layout-sider-header"
        style={{ borderBottom: `1px solid ${token.colorBorder}`, color: token.colorText }}
      >
        <span className="layout-sider-title">Spaces</span>
        <Button
          type="text"
          icon={<PlusOutlined />}
          size="small"
          onClick={() => setIsOpen(true)}
        />
      </div>
      <div className="layout-sider-body">
        <FolderTree
          onSelect={handleSelect}
          onSpaceDelete={handleDeleteSpace}
        />
      </div>
    </>
  );

  if (containerType === "panel") {
    return (
      <div
        className="layout-sider"
        style={{
          height: "100%",
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
