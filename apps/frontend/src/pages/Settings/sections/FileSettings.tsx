import { Card, Switch, Select, Typography, Space, Row, Col } from 'antd';
import { useSettingsStore, type SortBy, type SortOrder } from '@/stores/settingsStore';
import SettingSectionHeader from '../components/SettingSectionHeader';
import SettingRow from '../components/SettingRow';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

const FileSettings = () => {
  const { t } = useTranslation();
  const showHiddenFiles = useSettingsStore((state) => state.showHiddenFiles);
  const setShowHiddenFiles = useSettingsStore((state) => state.setShowHiddenFiles);
  const defaultSortBy = useSettingsStore((state) => state.defaultSortBy);
  const defaultSortOrder = useSettingsStore((state) => state.defaultSortOrder);
  const setDefaultSort = useSettingsStore((state) => state.setDefaultSort);

  return (
    <Space vertical size="small" className="settings-section">
      <SettingSectionHeader title={t('fileSettings.sectionTitle')} subtitle={t('fileSettings.sectionSubtitle')} />

      <Card size="small">
        <Space vertical size="small" className="settings-stack-full">
          <SettingRow
            left={<Text strong>{t('fileSettings.showHiddenFiles')}</Text>}
            right={(
              <Switch
                checked={showHiddenFiles}
                onChange={setShowHiddenFiles}
              />
            )}
          />

          <div>
            <div className="settings-field-title">
              <Text strong>{t('fileSettings.defaultSort')}</Text>
            </div>
            <Row gutter={8}>
              <Col span={12}>
                <Select
                  size="small"
                  className="settings-select-full"
                  value={defaultSortBy}
                  onChange={(value: string) => setDefaultSort(value as SortBy, defaultSortOrder)}
                  options={[
                    { value: 'name', label: t('fileSettings.sortName') },
                    { value: 'modTime', label: t('fileSettings.sortModified') },
                    { value: 'size', label: t('fileSettings.sortSize') },
                  ]}
                />
              </Col>
              <Col span={12}>
                <Select
                  size="small"
                  className="settings-select-full"
                  value={defaultSortOrder}
                  onChange={(value: string) => setDefaultSort(defaultSortBy, value as SortOrder)}
                  options={[
                    { value: 'ascend', label: t('fileSettings.sortAsc') },
                    { value: 'descend', label: t('fileSettings.sortDesc') },
                  ]}
                />
              </Col>
            </Row>
          </div>
        </Space>
      </Card>
    </Space>
  );
};

export default FileSettings;
