import DirectorySetupModal from "@/features/space/components/DirectorySetupModal";
import { PlusOutlined } from "@ant-design/icons";
import { Button, Layout, Menu, theme, type MenuProps } from "antd";
import type { ItemType } from "antd/es/menu/interface";
import { useState } from "react";

const { Sider } = Layout;

interface MainSiderProps {
  spaceItems: ItemType[];
}

export default function MainSider({ spaceItems }: MainSiderProps) {
  const { token } = theme.useToken();
  const [isOpen, setIsOpen] = useState(false);

  const spaceMenuItems: MenuProps['items'] = [
    {
      key: 'space',
      label: (
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <span>Space</span>
          <Button
            type="link"
            icon={<PlusOutlined />}
            size="small"
            onClick={() => setIsOpen(true)}
          ></Button>
        </div>
      ),
      type: 'group',
      children: spaceItems
    }
  ];

  return (
    <Sider
      style={{
        background: token.colorBgContainer
      }}
    >
      <DirectorySetupModal isOpen={isOpen} onClose={() => setIsOpen(false)} />
      <Menu mode="inline" items={spaceMenuItems} />
    </Sider>
  );
}