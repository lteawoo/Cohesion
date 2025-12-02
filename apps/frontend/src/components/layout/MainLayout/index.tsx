import { ConfigProvider, Layout, theme } from "antd";
import { Outlet } from "react-router";
import { MailOutlined } from "@ant-design/icons";
import MainSider from "./MainSider";
import type { ItemType } from "antd/es/menu/interface";
import { useState } from "react";

const { Header, Content } = Layout;

const items: ItemType[] = [
    { key: '1', icon: <MailOutlined />, label: 'My folder' },
]

export default function MainLayout() {
    const [isDarkMode, setIsDarkMode] = useState(true);

    const { token } = theme.useToken();

    const currentAlgorithm = isDarkMode ? theme.darkAlgorithm : theme.defaultAlgorithm;

    console.log(currentAlgorithm)

    return (
        <ConfigProvider theme={{ algorithm: currentAlgorithm }}>
            <Layout
                style={{
                    display: 'flex',
                    minHeight: '100vh',
                    overflow: 'hidden'
                }}
            >
                <Header
                    style={{
                        background: token.colorBgContainer
                    }}
                >
                    <div style={{ color: token.colorText, fontSize: '20px' }}>
                        Cohesion
                    </div>
                </Header>
                <Layout>
                    <MainSider spaceItems={items} />

                    <Content>
                        <main style={{ flex: 1, overflowY: 'auto' }}>
                            <Outlet />
                        </main>
                    </Content>
                </Layout>
            </Layout>
        </ConfigProvider>
    )
}