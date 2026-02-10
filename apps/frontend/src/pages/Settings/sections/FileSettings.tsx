import { Card, Switch, Select, Typography, Space, Row, Col } from 'antd';
import { useSettingsStore } from '@/stores/settingsStore';

const { Title, Text } = Typography;

const FileSettings = () => {
  const showHiddenFiles = useSettingsStore((state) => state.showHiddenFiles);
  const setShowHiddenFiles = useSettingsStore((state) => state.setShowHiddenFiles);
  const defaultSortBy = useSettingsStore((state) => state.defaultSortBy);
  const defaultSortOrder = useSettingsStore((state) => state.defaultSortOrder);
  const setDefaultSort = useSettingsStore((state) => state.setDefaultSort);

  return (
    <Space direction="vertical" size="large" style={{ width: '100%', maxWidth: 600 }}>
      <div>
        <Title level={3}>파일 설정</Title>
        <Text type="secondary">파일 브라우저 동작 설정</Text>
      </div>

      <Card>
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <div>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <div>
                <Text strong>숨김 파일 표시</Text>
                <div style={{ marginTop: 4 }}>
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    .으로 시작하는 숨김 파일을 표시합니다.
                  </Text>
                </div>
              </div>
              <Switch
                checked={showHiddenFiles}
                onChange={setShowHiddenFiles}
              />
            </div>
          </div>

          <div>
            <Text strong>기본 정렬</Text>
            <div style={{ marginTop: 8 }}>
              <Row gutter={8}>
                <Col span={12}>
                  <Select
                    style={{ width: '100%' }}
                    value={defaultSortBy}
                    onChange={(value) => setDefaultSort(value, defaultSortOrder)}
                    options={[
                      { value: 'name', label: '이름' },
                      { value: 'modTime', label: '수정일' },
                      { value: 'size', label: '크기' },
                    ]}
                  />
                </Col>
                <Col span={12}>
                  <Select
                    style={{ width: '100%' }}
                    value={defaultSortOrder}
                    onChange={(value) => setDefaultSort(defaultSortBy, value)}
                    options={[
                      { value: 'ascend', label: '오름차순' },
                      { value: 'descend', label: '내림차순' },
                    ]}
                  />
                </Col>
              </Row>
            </div>
            <Text type="secondary" style={{ fontSize: 12, marginTop: 4, display: 'block' }}>
              파일 목록의 기본 정렬 방식을 선택합니다. (폴더는 항상 먼저 표시)
            </Text>
          </div>
        </Space>
      </Card>
    </Space>
  );
};

export default FileSettings;
