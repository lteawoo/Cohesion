import { Card, Radio, Typography, Space } from 'antd';
import { useSettingsStore } from '@/stores/settingsStore';

const { Title, Text } = Typography;

const AppearanceSettings = () => {
  const theme = useSettingsStore((state) => state.theme);
  const setTheme = useSettingsStore((state) => state.setTheme);
  const defaultViewMode = useSettingsStore((state) => state.defaultViewMode);
  const setDefaultViewMode = useSettingsStore((state) => state.setDefaultViewMode);

  return (
    <Space vertical size="small" style={{ width: '100%', maxWidth: 480 }}>
      <div>
        <Title level={4} style={{ margin: 0 }}>외관 설정</Title>
        <Text type="secondary" style={{ fontSize: 13 }}>테마 및 보기 옵션</Text>
      </div>

      <Card size="small">
        <Space vertical size="small" style={{ width: '100%' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <Text strong>테마</Text>
            <Radio.Group
              value={theme}
              onChange={(e) => setTheme(e.target.value)}
              buttonStyle="solid"
              size="small"
            >
              <Radio.Button value="light">라이트</Radio.Button>
              <Radio.Button value="dark">다크</Radio.Button>
            </Radio.Group>
          </div>

          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <Text strong>기본 뷰 모드</Text>
            <Radio.Group
              value={defaultViewMode}
              onChange={(e) => setDefaultViewMode(e.target.value)}
              buttonStyle="solid"
              size="small"
            >
              <Radio.Button value="grid">그리드</Radio.Button>
              <Radio.Button value="table">테이블</Radio.Button>
            </Radio.Group>
          </div>
        </Space>
      </Card>
    </Space>
  );
};

export default AppearanceSettings;
