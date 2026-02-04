import DirectorySetupModal from "@/features/space/components/DirectorySetupModal";
import FolderTree from "@/features/browse/components/FolderTree";
import { PlusOutlined } from "@ant-design/icons";
import { Button, Layout, theme } from "antd";
import type { Space } from "@/features/space/types";
import { useState } from "react";

const { Sider } = Layout;

interface MainSiderProps {
  spaces: Space[];
  onSpaceCreated?: () => void;
  onPathSelect?: (path: string) => void;
}

export default function MainSider({ spaces, onSpaceCreated, onPathSelect }: MainSiderProps) {
  const { token } = theme.useToken();
  const [isOpen, setIsOpen] = useState(false);

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
        />
      </div>
    </Sider>
  );
}