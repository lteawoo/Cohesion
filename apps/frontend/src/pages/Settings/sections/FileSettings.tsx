import { Card, Switch, Select, Typography, Space, Row, Col } from 'antd';
import { useSettingsStore, type SortBy, type SortOrder } from '@/stores/settingsStore';
import SettingSectionHeader from '../components/SettingSectionHeader';
import SettingRow from '../components/SettingRow';

const { Text } = Typography;

const FileSettings = () => {
  const showHiddenFiles = useSettingsStore((state) => state.showHiddenFiles);
  const setShowHiddenFiles = useSettingsStore((state) => state.setShowHiddenFiles);
  const defaultSortBy = useSettingsStore((state) => state.defaultSortBy);
  const defaultSortOrder = useSettingsStore((state) => state.defaultSortOrder);
  const setDefaultSort = useSettingsStore((state) => state.setDefaultSort);

  return (
    <Space vertical size="small" className="settings-section">
      <SettingSectionHeader title="파일 설정" subtitle="파일 브라우저 동작 설정" />

      <Card size="small">
        <Space vertical size="small" className="settings-stack-full">
          <SettingRow
            left={<Text strong>숨김 파일 표시</Text>}
            right={(
              <Switch
                checked={showHiddenFiles}
                onChange={setShowHiddenFiles}
              />
            )}
          />

          <div>
            <div className="settings-field-title">
              <Text strong>기본 정렬</Text>
            </div>
            <Row gutter={8}>
              <Col span={12}>
                <Select
                  size="small"
                  style={{ width: '100%' }}
                  value={defaultSortBy}
                  onChange={(value: string) => setDefaultSort(value as SortBy, defaultSortOrder)}
                  options={[
                    { value: 'name', label: '이름' },
                    { value: 'modTime', label: '수정일' },
                    { value: 'size', label: '크기' },
                  ]}
                />
              </Col>
              <Col span={12}>
                <Select
                  size="small"
                  style={{ width: '100%' }}
                  value={defaultSortOrder}
                  onChange={(value: string) => setDefaultSort(defaultSortBy, value as SortOrder)}
                  options={[
                    { value: 'ascend', label: '오름차순' },
                    { value: 'descend', label: '내림차순' },
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
