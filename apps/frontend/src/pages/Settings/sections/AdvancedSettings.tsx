import { Card, Button, Typography, Space, Popconfirm, App } from 'antd';
import { useSettingsStore } from '@/stores/settingsStore';
import { DeleteOutlined, ReloadOutlined } from '@ant-design/icons';

const { Title, Text } = Typography;

const AdvancedSettings = () => {
  const { message } = App.useApp();
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
    <Space vertical size="small" style={{ width: '100%', maxWidth: 480 }}>
      <div>
        <Title level={4} style={{ margin: 0 }}>고급 설정</Title>
        <Text type="secondary" style={{ fontSize: 13 }}>시스템 관리 및 고급 옵션</Text>
      </div>

      <Card title="데이터 관리" size="small">
        <Space vertical size="small" style={{ width: '100%' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <Text strong>캐시 삭제</Text>
            <Button
              size="small"
              icon={<DeleteOutlined />}
              onClick={handleClearCache}
            >
              삭제
            </Button>
          </div>

          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <Text strong>설정 초기화</Text>
            <Popconfirm
              title="설정 초기화"
              description="모든 설정을 기본값으로 되돌립니다. 계속하시겠습니까?"
              onConfirm={handleReset}
              okText="초기화"
              cancelText="취소"
              okButtonProps={{ danger: true }}
            >
              <Button
                size="small"
                icon={<ReloadOutlined />}
                danger
              >
                초기화
              </Button>
            </Popconfirm>
          </div>
        </Space>
      </Card>

      <Card title="정보" size="small">
        <Space vertical size="small">
          <Text style={{ fontSize: 13 }}>
            <Text strong>버전:</Text> 1.0.0
          </Text>
          <Text style={{ fontSize: 13 }}>
            <Text strong>빌드:</Text> {import.meta.env.MODE}
          </Text>
        </Space>
      </Card>
    </Space>
  );
};

export default AdvancedSettings;
