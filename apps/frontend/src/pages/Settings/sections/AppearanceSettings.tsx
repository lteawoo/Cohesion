import { Card, Radio, Typography, Space } from 'antd';
import { useSettingsStore } from '@/stores/settingsStore';
import SettingSectionHeader from '../components/SettingSectionHeader';
import SettingRow from '../components/SettingRow';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

const AppearanceSettings = () => {
  const { t } = useTranslation();
  const theme = useSettingsStore((state) => state.theme);
  const setTheme = useSettingsStore((state) => state.setTheme);

  return (
    <Space vertical size="small" className="settings-section">
      <SettingSectionHeader title={t('appearanceSettings.sectionTitle')} subtitle={t('appearanceSettings.sectionSubtitle')} />

      <Card size="small">
        <Space vertical size="small" className="settings-stack-full">
          <SettingRow
            left={<Text strong>{t('appearanceSettings.themeLabel')}</Text>}
            right={(
              <Radio.Group
                value={theme}
                onChange={(e) => setTheme(e.target.value)}
                buttonStyle="solid"
                size="small"
              >
                <Radio.Button value="light">{t('appearanceSettings.themeLight')}</Radio.Button>
                <Radio.Button value="dark">{t('appearanceSettings.themeDark')}</Radio.Button>
              </Radio.Group>
            )}
          />
        </Space>
      </Card>
    </Space>
  );
};

export default AppearanceSettings;
