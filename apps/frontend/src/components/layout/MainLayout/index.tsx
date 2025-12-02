import { Button, Layout, Menu } from "antd";
import type { MenuProps } from 'antd';
import { Outlet } from "react-router";
import { MailOutlined, PlusOutlined } from "@ant-design/icons";

const { Header, Sider, Content } = Layout;
type MenuItem = Required<MenuProps>['items'][number];

const items: MenuItem[] = [
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
        children: [
            { key: '1', icon: <MailOutlined />, label: 'My folder' },
        ],
    }
]

export default function MainLayout() {
    return (
        <Layout
            style={{
                display: 'flex',
                minHeight: '100vh',
                overflow: 'hidden'
            }}
        >
            <Header
                style={{

                }}
            >
                <div style={{ color: 'white', fontSize: '20px' }}>
                    Cohesion
                </div>
            </Header>
            <Layout>
                <Sider
                    style={{

                    }}
                >
                    <Menu theme="dark" mode="inline" defaultSelectedKeys={['4']} items={items} />
                </Sider>

                <Content>
                    <main style={{ flex: 1, overflowY: 'auto' }}>
                        <Outlet />
                    </main>
                </Content>
            </Layout>
        </Layout>
    )
}