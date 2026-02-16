import React from 'react';
import { Typography } from 'antd';

const { Title, Text } = Typography;

interface SettingSectionHeaderProps {
  title: string;
  subtitle: string;
}

const SettingSectionHeader: React.FC<SettingSectionHeaderProps> = ({ title, subtitle }) => {
  return (
    <div>
      <Title level={4} className="settings-section-title">{title}</Title>
      <Text type="secondary" className="settings-section-subtitle">{subtitle}</Text>
    </div>
  );
};

export default SettingSectionHeader;
