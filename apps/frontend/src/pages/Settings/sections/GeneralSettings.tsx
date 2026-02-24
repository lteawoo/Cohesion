import { Card, Select, Typography, Space, Button, Popconfirm, App } from 'antd';
import { useSettingsStore } from '@/stores/settingsStore';
import { ReloadOutlined } from '@ant-design/icons';
import SettingSectionHeader from '../components/SettingSectionHeader';
import SettingRow from '../components/SettingRow';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

const GeneralSettings = () => {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const language = useSettingsStore((state) => state.language);
  const setLanguage = useSettingsStore((state) => state.setLanguage);
  const resetToDefaults = useSettingsStore((state) => state.resetToDefaults);

  const handleReset = () => {
    resetToDefaults();
    message.success(t('generalSettings.resetSuccess'));
  };

  return (
    <Space vertical size="small" className="settings-section">
      <SettingSectionHeader
        title={t('generalSettings.sectionTitle')}
        subtitle={t('generalSettings.sectionSubtitle')}
      />

      <Card size="small" title={t('generalSettings.baseCardTitle')}>
        <Space vertical size={4} className="settings-stack-full">
          <Text type="secondary">{t('generalSettings.baseDescription')}</Text>
          <SettingRow
            left={<Text strong>{t('generalSettings.languageLabel')}</Text>}
            right={(
              <Select
                className="settings-select-compact"
                value={language}
                onChange={setLanguage}
                options={[
                  { value: 'ko', label: t('generalSettings.languageKo') },
                  { value: 'en', label: t('generalSettings.languageEn') },
                ]}
              />
            )}
          />
        </Space>
      </Card>

      <Card size="small" title={t('generalSettings.dataCardTitle')}>
        <Space vertical size={4} className="settings-stack-full">
          <Text type="secondary">{t('generalSettings.dataDescription')}</Text>

          <SettingRow
            left={<Text strong>{t('generalSettings.resetLabel')}</Text>}
            right={(
              <Popconfirm
                title={t('generalSettings.resetTitle')}
                description={t('generalSettings.resetDescription')}
                onConfirm={handleReset}
                okText={t('generalSettings.resetOk')}
                cancelText={t('generalSettings.resetCancel')}
                okButtonProps={{ danger: true }}
              >
                <Button
                  size="small"
                  icon={<ReloadOutlined />}
                  danger
                >
                  {t('generalSettings.resetButton')}
                </Button>
              </Popconfirm>
            )}
          />
        </Space>
      </Card>
    </Space>
  );
};

export default GeneralSettings;
