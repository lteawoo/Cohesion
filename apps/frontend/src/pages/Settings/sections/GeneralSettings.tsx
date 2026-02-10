import { Card, Select, Typography, Space } from 'antd';
import { useSettingsStore } from '@/stores/settingsStore';

const { Title, Text } = Typography;

const GeneralSettings = () => {
  const language = useSettingsStore((state) => state.language);
  const setLanguage = useSettingsStore((state) => state.setLanguage);

  return (
    <Space vertical size="small" style={{ width: '100%', maxWidth: 480 }}>
      <div>
        <Title level={4} style={{ margin: 0 }}>일반 설정</Title>
        <Text type="secondary" style={{ fontSize: 13 }}>기본 애플리케이션 설정</Text>
      </div>

      <Card size="small">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <Text strong>언어</Text>
          <Select
            style={{ width: 140 }}
            value={language}
            onChange={setLanguage}
            options={[
              { value: 'ko', label: '한국어' },
              { value: 'en', label: 'English' },
            ]}
          />
        </div>
      </Card>
    </Space>
  );
};

export default GeneralSettings;
