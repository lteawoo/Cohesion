import { Card, Select, Typography, Space } from 'antd';
import { useSettingsStore } from '@/stores/settingsStore';
import SettingSectionHeader from '../components/SettingSectionHeader';
import SettingRow from '../components/SettingRow';

const { Text } = Typography;

const GeneralSettings = () => {
  const language = useSettingsStore((state) => state.language);
  const setLanguage = useSettingsStore((state) => state.setLanguage);

  return (
    <Space vertical size="small" className="settings-section">
      <SettingSectionHeader title="일반 설정" subtitle="기본 애플리케이션 설정" />

      <Card size="small">
        <SettingRow
          left={<Text strong>언어</Text>}
          right={(
            <Select
              className="settings-select-compact"
              value={language}
              onChange={setLanguage}
              options={[
                { value: 'ko', label: '한국어' },
                { value: 'en', label: 'English' },
              ]}
            />
          )}
        />
      </Card>
    </Space>
  );
};

export default GeneralSettings;
