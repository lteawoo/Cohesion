import { PlusOutlined } from "@ant-design/icons";
import { Button, Layout, Menu, type MenuProps } from "antd";
import type { ItemType } from "antd/es/menu/interface";

const { Sider } = Layout;

interface MainSiderProps {
    spaceItems: ItemType[];
}

export default function MainSider({ spaceItems }: MainSiderProps) {

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

            }}
        >
            <Menu mode="inline" items={spaceMenuItems} />
        </Sider>
    );
}