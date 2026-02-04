import DirectorySetupModal from "@/features/space/components/DirectorySetupModal";
import FolderTree from "@/features/browse/components/FolderTree";
import { PlusOutlined } from "@ant-design/icons";
import { Button, Layout, theme, Modal, message } from "antd";
import type { Space } from "@/features/space/types";
import { useState } from "react";
import { useDeleteSpace } from "@/features/space/hooks/useDeleteSpace";

const { Sider } = Layout;

interface MainSiderProps {
  spaces: Space[];
  onSpaceCreated?: () => void;
  onPathSelect?: (path: string, space?: Space) => void;
}

export default function MainSider({ spaces, onSpaceCreated, onPathSelect }: MainSiderProps) {
  const { token } = theme.useToken();
  const [isOpen, setIsOpen] = useState(false);
  const { deleteSpace, isLoading: isDeleting } = useDeleteSpace();

  const handleDeleteSpace = (space: Space) => {
    Modal.confirm({
      title: 'Space 삭제',
      content: `"${space.space_name}" Space를 삭제하시겠습니까? 이 작업은 되돌릴 수 없습니다.`,
      okText: '삭제',
      cancelText: '취소',
      okButtonProps: { danger: true, loading: isDeleting },
      onOk: async () => {
        try {
          await deleteSpace(space.id);
          message.success('Space가 삭제되었습니다.');
          onSpaceCreated?.(); // 트리 갱신
        } catch (error) {
          message.error(`Space 삭제 실패: ${error instanceof Error ? error.message : '알 수 없는 오류'}`);
        }
      },
    });
  };

  return (
    <Sider
      width={300}
      style={{
        background: token.colorBgContainer,
        overflow: 'auto'
      }}
    >
      <DirectorySetupModal
        isOpen={isOpen}
        onClose={() => setIsOpen(false)}
        onSuccess={onSpaceCreated}
      />
      <div style={{
        padding: '12px 16px',
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
        borderBottom: `1px solid ${token.colorBorder}`
      }}>
        <span style={{ fontWeight: 'bold', fontSize: '14px', color: token.colorText }}>Spaces</span>
        <Button
          type="text"
          icon={<PlusOutlined />}
          size="small"
          onClick={() => setIsOpen(true)}
        />
      </div>
      <div style={{ padding: '8px' }}>
        <FolderTree
          onSelect={onPathSelect || (() => {})}
          spaces={spaces}
          onSpaceDelete={handleDeleteSpace}
        />
      </div>
    </Sider>
  );
}