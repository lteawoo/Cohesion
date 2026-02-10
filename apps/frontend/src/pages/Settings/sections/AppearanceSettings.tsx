import { Card, Radio, Typography, Space } from 'antd';
import { useSettingsStore } from '@/stores/settingsStore';

const { Title, Text } = Typography;

const AppearanceSettings = () => {
  const theme = useSettingsStore((state) => state.theme);
  const setTheme = useSettingsStore((state) => state.setTheme);
  const defaultViewMode = useSettingsStore((state) => state.defaultViewMode);
  const setDefaultViewMode = useSettingsStore((state) => state.setDefaultViewMode);

  return (
    <Space direction="vertical" size="large" style={{ width: '100%', maxWidth: 600 }}>
      <div>
        <Title level={3}>외관 설정</Title>
        <Text type="secondary">테마 및 보기 옵션</Text>
      </div>

      <Card>
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <div>
            <Text strong>테마</Text>
            <div style={{ marginTop: 8 }}>
              <Radio.Group
                value={theme}
                onChange={(e) => setTheme(e.target.value)}
                buttonStyle="solid"
              >
                <Radio.Button value="light">라이트</Radio.Button>
                <Radio.Button value="dark">다크</Radio.Button>
              </Radio.Group>
            </div>
            <Text type="secondary" style={{ fontSize: 12, marginTop: 4, display: 'block' }}>
              애플리케이션 테마를 선택합니다.
            </Text>
          </div>

          <div>
            <Text strong>기본 뷰 모드</Text>
            <div style={{ marginTop: 8 }}>
              <Radio.Group
                value={defaultViewMode}
                onChange={(e) => setDefaultViewMode(e.target.value)}
                buttonStyle="solid"
              >
                <Radio.Button value="grid">그리드</Radio.Button>
                <Radio.Button value="table">테이블</Radio.Button>
              </Radio.Group>
            </div>
            <Text type="secondary" style={{ fontSize: 12, marginTop: 4, display: 'block' }}>
              파일 브라우저의 기본 표시 방식을 선택합니다.
            </Text>
          </div>
        </Space>
      </Card>
    </Space>
  );
};

export default AppearanceSettings;
