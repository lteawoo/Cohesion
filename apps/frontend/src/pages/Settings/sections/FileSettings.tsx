import { Card, Switch, Select, Typography, Space, Row, Col } from 'antd';
import { useSettingsStore, type SortBy, type SortOrder } from '@/stores/settingsStore';

const { Title, Text } = Typography;

const FileSettings = () => {
  const showHiddenFiles = useSettingsStore((state) => state.showHiddenFiles);
  const setShowHiddenFiles = useSettingsStore((state) => state.setShowHiddenFiles);
  const defaultSortBy = useSettingsStore((state) => state.defaultSortBy);
  const defaultSortOrder = useSettingsStore((state) => state.defaultSortOrder);
  const setDefaultSort = useSettingsStore((state) => state.setDefaultSort);

  return (
    <Space vertical size="small" style={{ width: '100%', maxWidth: 480 }}>
      <div>
        <Title level={4} style={{ margin: 0 }}>파일 설정</Title>
        <Text type="secondary" style={{ fontSize: 13 }}>파일 브라우저 동작 설정</Text>
      </div>

      <Card size="small">
        <Space vertical size="small" style={{ width: '100%' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <Text strong>숨김 파일 표시</Text>
            <Switch
              checked={showHiddenFiles}
              onChange={setShowHiddenFiles}
            />
          </div>

          <div>
            <div style={{ marginBottom: 4 }}>
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
