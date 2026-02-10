import { Card, Select, Typography, Space } from 'antd';
import { useSettingsStore } from '@/stores/settingsStore';

const { Title, Text } = Typography;

const GeneralSettings = () => {
  const language = useSettingsStore((state) => state.language);
  const setLanguage = useSettingsStore((state) => state.setLanguage);

  return (
    <Space direction="vertical" size="large" style={{ width: '100%', maxWidth: 600 }}>
      <div>
        <Title level={3}>일반 설정</Title>
        <Text type="secondary">기본 애플리케이션 설정</Text>
      </div>

      <Card>
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <div>
            <Text strong>언어</Text>
            <div style={{ marginTop: 8 }}>
              <Select
                style={{ width: '100%' }}
                value={language}
                onChange={setLanguage}
                options={[
                  { value: 'ko', label: '한국어' },
                  { value: 'en', label: 'English' },
                ]}
              />
            </div>
            <Text type="secondary" style={{ fontSize: 12, marginTop: 4, display: 'block' }}>
              애플리케이션 인터페이스 언어를 선택합니다.
            </Text>
          </div>
        </Space>
      </Card>
    </Space>
  );
};

export default GeneralSettings;
