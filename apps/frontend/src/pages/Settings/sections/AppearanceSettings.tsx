import { Card, Radio, Typography, Space } from 'antd';
import { useSettingsStore } from '@/stores/settingsStore';
import SettingSectionHeader from '../components/SettingSectionHeader';
import SettingRow from '../components/SettingRow';

const { Text } = Typography;

const AppearanceSettings = () => {
  const theme = useSettingsStore((state) => state.theme);
  const setTheme = useSettingsStore((state) => state.setTheme);
  const defaultViewMode = useSettingsStore((state) => state.defaultViewMode);
  const setDefaultViewMode = useSettingsStore((state) => state.setDefaultViewMode);

  return (
    <Space vertical size="small" className="settings-section">
      <SettingSectionHeader title="외관 설정" subtitle="테마 및 보기 옵션" />

      <Card size="small">
        <Space vertical size="small" className="settings-stack-full">
          <SettingRow
            left={<Text strong>테마</Text>}
            right={(
              <Radio.Group
                value={theme}
                onChange={(e) => setTheme(e.target.value)}
                buttonStyle="solid"
                size="small"
              >
                <Radio.Button value="light">라이트</Radio.Button>
                <Radio.Button value="dark">다크</Radio.Button>
              </Radio.Group>
            )}
          />

          <SettingRow
            left={<Text strong>기본 뷰 모드</Text>}
            right={(
              <Radio.Group
                value={defaultViewMode}
                onChange={(e) => setDefaultViewMode(e.target.value)}
                buttonStyle="solid"
                size="small"
              >
                <Radio.Button value="grid">그리드</Radio.Button>
                <Radio.Button value="table">테이블</Radio.Button>
              </Radio.Group>
            )}
          />
        </Space>
      </Card>
    </Space>
  );
};

export default AppearanceSettings;
