import { Layout, Menu, type MenuProps } from "antd";
import {
  AppstoreOutlined,
  BarChartOutlined,
  CloudOutlined,
  ShopOutlined,
  TeamOutlined,
  UploadOutlined,
  UserOutlined,
  VideoCameraOutlined,
} from '@ant-design/icons';
import { Outlet } from "react-router";
import React from "react";

const { Header, Sider, Content } = Layout;

const items: MenuProps['items'] = [
  UserOutlined,
  VideoCameraOutlined,
  UploadOutlined,
  BarChartOutlined,
  CloudOutlined,
  AppstoreOutlined,
  TeamOutlined,
  ShopOutlined,
].map((icon, index) => ({
  key: String(index + 1),
  icon: React.createElement(icon),
  label: `nav ${index + 1}`,
}));

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