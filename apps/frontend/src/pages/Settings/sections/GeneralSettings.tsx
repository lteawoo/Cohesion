import { Card, Select, Typography, Space, Button, Popconfirm, App } from 'antd';
import { useSettingsStore } from '@/stores/settingsStore';
import { DeleteOutlined, ReloadOutlined } from '@ant-design/icons';
import SettingSectionHeader from '../components/SettingSectionHeader';
import SettingRow from '../components/SettingRow';

const { Text } = Typography;

const GeneralSettings = () => {
  const { message } = App.useApp();
  const language = useSettingsStore((state) => state.language);
  const setLanguage = useSettingsStore((state) => state.setLanguage);
  const resetToDefaults = useSettingsStore((state) => state.resetToDefaults);

  const handleReset = () => {
    resetToDefaults();
    message.success('설정이 초기화되었습니다');
  };

  const handleClearCache = () => {
    message.success('캐시가 삭제되었습니다');
  };

  return (
    <Space vertical size="small" className="settings-section">
      <SettingSectionHeader title="일반 설정" subtitle="기본 애플리케이션 설정" />

      <Card size="small" title="기본">
        <Space vertical size={4} className="settings-stack-full">
          <Text type="secondary">언어, 기본 동작과 같은 일반 옵션을 설정합니다.</Text>
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
        </Space>
      </Card>

      <Card size="small" title="데이터 관리">
        <Space vertical size={4} className="settings-stack-full">
          <Text type="secondary">브라우저 저장 설정을 정리하거나 기본값으로 되돌릴 수 있습니다.</Text>
          <SettingRow
            left={<Text strong>캐시 삭제</Text>}
            right={(
              <Button
                size="small"
                icon={<DeleteOutlined />}
                onClick={handleClearCache}
              >
                삭제
              </Button>
            )}
          />

          <SettingRow
            left={<Text strong>설정 초기화</Text>}
            right={(
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
            )}
          />
        </Space>
      </Card>
    </Space>
  );
};

export default GeneralSettings;
