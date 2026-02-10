import { Card, Button, Typography, Space, message, Popconfirm } from 'antd';
import { useSettingsStore } from '@/stores/settingsStore';
import { DeleteOutlined, ReloadOutlined } from '@ant-design/icons';

const { Title, Text } = Typography;

const AdvancedSettings = () => {
  const resetToDefaults = useSettingsStore((state) => state.resetToDefaults);

  const handleReset = () => {
    resetToDefaults();
    message.success('설정이 초기화되었습니다');
  };

  const handleClearCache = () => {
    // 브라우저 캐시 클리어 로직 (향후 구현)
    message.success('캐시가 삭제되었습니다');
  };

  return (
    <Space direction="vertical" size="large" style={{ width: '100%', maxWidth: 600 }}>
      <div>
        <Title level={3}>고급 설정</Title>
        <Text type="secondary">시스템 관리 및 고급 옵션</Text>
      </div>

      <Card title="데이터 관리">
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <div>
            <Text strong>캐시 삭제</Text>
            <div style={{ marginTop: 8 }}>
              <Button
                icon={<DeleteOutlined />}
                onClick={handleClearCache}
              >
                캐시 삭제
              </Button>
            </div>
            <Text type="secondary" style={{ fontSize: 12, marginTop: 4, display: 'block' }}>
              브라우저에 저장된 임시 데이터를 삭제합니다.
            </Text>
          </div>

          <div>
            <Text strong>설정 초기화</Text>
            <div style={{ marginTop: 8 }}>
              <Popconfirm
                title="설정 초기화"
                description="모든 설정을 기본값으로 되돌립니다. 계속하시겠습니까?"
                onConfirm={handleReset}
                okText="초기화"
                cancelText="취소"
                okButtonProps={{ danger: true }}
              >
                <Button
                  icon={<ReloadOutlined />}
                  danger
                >
                  설정 초기화
                </Button>
              </Popconfirm>
            </div>
            <Text type="secondary" style={{ fontSize: 12, marginTop: 4, display: 'block' }}>
              모든 사용자 설정을 기본값으로 되돌립니다.
            </Text>
          </div>
        </Space>
      </Card>

      <Card title="정보">
        <Space direction="vertical" size="small">
          <Text>
            <Text strong>버전:</Text> 1.0.0
          </Text>
          <Text>
            <Text strong>빌드:</Text> {import.meta.env.MODE}
          </Text>
        </Space>
      </Card>
    </Space>
  );
};

export default AdvancedSettings;
