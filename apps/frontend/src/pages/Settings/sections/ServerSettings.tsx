import { Card, Switch, InputNumber, Typography, Space, Alert, Divider, Button, App } from 'antd';
import { ReloadOutlined, SaveOutlined } from '@ant-design/icons';
import { useState, useEffect, useCallback } from 'react';
import { getConfig, updateConfig, restartServer, waitForReconnect, type Config } from '@/api/config';

const { Title, Text } = Typography;

const ServerSettings = () => {
  const { message, modal } = App.useApp();
  const [config, setConfig] = useState<Config | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [restarting, setRestarting] = useState(false);
  const [hasChanges, setHasChanges] = useState(false);

  // 개발 모드 여부
  const isDev = import.meta.env.DEV;

  const loadConfig = useCallback(async () => {
    try {
      const data = await getConfig();
      setConfig(data);
      setHasChanges(false);
    } catch {
      message.error('설정을 불러오는데 실패했습니다');
    } finally {
      setLoading(false);
    }
  }, [message]);

  // 초기 설정 로드
  useEffect(() => {
    loadConfig();
  }, [loadConfig]);

  const handleSave = async () => {
    if (!config) return;

    setSaving(true);
    try {
      await updateConfig(config);
      message.success('설정이 저장되었습니다');
      setHasChanges(false);
    } catch {
      message.error('설정 저장에 실패했습니다');
    } finally {
      setSaving(false);
    }
  };

  const handleRestart = async () => {
    modal.confirm({
      title: '서버 재시작',
      content: '서버를 재시작하시겠습니까? 잠시 연결이 끊어질 수 있습니다.',
      okText: '재시작',
      cancelText: '취소',
      onOk: async () => {
        if (!config) return;

        setRestarting(true);
        try {
          // 먼저 저장
          if (hasChanges) {
            await updateConfig(config);
          }

          const newPort = await restartServer();

          message.loading({
            content: '서버 재시작 중...',
            key: 'restart',
            duration: 0,
          });

          // 잠시 대기 (서버가 종료될 시간)
          await new Promise(resolve => setTimeout(resolve, 2000));

          if (isDev) {
            // 개발 모드: 프론트엔드(5173)와 백엔드(3000)가 분리되어 있음
            // 백엔드 포트가 변경되어도 Vite proxy가 처리하므로 현재 페이지만 새로고침

            const success = await waitForReconnect();

            if (success) {
              message.success({
                content: '서버가 재시작되었습니다',
                key: 'restart',
              });
              setTimeout(() => {
                window.location.reload();
              }, 500);
            } else {
              message.error({
                content: '서버 재시작 실패 또는 타임아웃',
                key: 'restart',
              });
            }
          } else {
            // 프로덕션 모드: 프론트엔드와 백엔드가 같은 서버
            // 포트가 변경되었으면 새 포트로 리다이렉트
            const currentPort = window.location.port || '80';

            if (currentPort === newPort) {
              // 같은 포트
              const success = await waitForReconnect();

              if (success) {
                message.success({
                  content: '서버가 재시작되었습니다',
                  key: 'restart',
                });
                setTimeout(() => {
                  window.location.reload();
                }, 500);
              } else {
                message.error({
                  content: '서버 재시작 실패 또는 타임아웃',
                  key: 'restart',
                });
              }
            } else {
              // 다른 포트: 리다이렉트
              message.success({
                content: `서버가 포트 ${newPort}에서 재시작되었습니다`,
                key: 'restart',
              });

              setTimeout(() => {
                const protocol = window.location.protocol;
                const hostname = window.location.hostname;
                window.location.href = `${protocol}//${hostname}:${newPort}/settings`;
              }, 1000);
            }
          }
        } catch (error) {
          message.error({
            content: `재시작 요청 실패: ${error instanceof Error ? error.message : '알 수 없는 오류'}`,
            key: 'restart',
          });
        } finally {
          setRestarting(false);
        }
      },
    });
  };

  const updateServerConfig = <K extends keyof Config['server']>(
    key: K,
    value: Config['server'][K]
  ) => {
    if (!config) return;
    setConfig({
      ...config,
      server: {
        ...config.server,
        [key]: value,
      },
    });
    setHasChanges(true);
  };

  if (loading || !config) {
    return <div>로딩 중...</div>;
  }

  const { server } = config;

  return (
    <Space vertical size="small" style={{ width: '100%', maxWidth: 480 }}>
      <div>
        <Title level={4} style={{ margin: 0 }}>서버 설정</Title>
        <Text type="secondary" style={{ fontSize: 13 }}>서버 및 프로토콜 설정</Text>
      </div>

      <Alert
        title="변경 후 재시작 필요"
        type="warning"
        showIcon
        style={{ padding: '8px 12px' }}
      />

      <Space size="small">
        <Button
          icon={<SaveOutlined />}
          onClick={handleSave}
          loading={saving}
          disabled={!hasChanges}
          size="small"
          type="primary"
        >
          저장
        </Button>
        <Button
          icon={<ReloadOutlined />}
          onClick={handleRestart}
          loading={restarting}
          size="small"
        >
          재시작
        </Button>
      </Space>

      <Card title="HTTP 서버" size="small">
        <Space vertical size="small" style={{ width: '100%' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <Text strong>활성화</Text>
            <Switch
              checked={server.httpEnabled}
              onChange={(checked: boolean) => updateServerConfig('httpEnabled', checked)}
            />
          </div>

          {server.httpEnabled && (
            <>
              <Divider style={{ margin: '6px 0' }} />
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <Text strong>포트</Text>
                <InputNumber
                  size="small"
                  min={1}
                  max={65535}
                  value={parseInt(server.port)}
                  onChange={(value: number | null) => value && updateServerConfig('port', value.toString())}
                  style={{ width: 100 }}
                />
              </div>
            </>
          )}
        </Space>
      </Card>

      <Card title="WebDAV" size="small">
        <Space vertical size="small" style={{ width: '100%' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <Text strong>활성화</Text>
            <Switch
              checked={server.webdavEnabled}
              onChange={(checked: boolean) => updateServerConfig('webdavEnabled', checked)}
              disabled={!server.httpEnabled}
            />
          </div>

          {server.webdavEnabled && server.httpEnabled && (
            <>
              <Divider style={{ margin: '6px 0' }} />
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <Text strong>경로</Text>
                <Text code>/dav/</Text>
              </div>
              <Text type="secondary" style={{ fontSize: 12 }}>
                HTTP 포트 {server.port} 사용
              </Text>
            </>
          )}
        </Space>
      </Card>

      <Card title="FTP 서버" size="small">
        <Space vertical size="small" style={{ width: '100%' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <div>
              <Text strong>활성화</Text>
              <Text type="secondary" style={{ fontSize: 12, marginLeft: 8 }}>
                (구현 예정)
              </Text>
            </div>
            <Switch
              checked={server.ftpEnabled}
              onChange={(checked: boolean) => updateServerConfig('ftpEnabled', checked)}
            />
          </div>

          {server.ftpEnabled && (
            <>
              <Divider style={{ margin: '6px 0' }} />
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <Text strong>포트</Text>
                <InputNumber
                  size="small"
                  min={1}
                  max={65535}
                  value={server.ftpPort}
                  onChange={(value: number | null) => value && updateServerConfig('ftpPort', value)}
                  style={{ width: 100 }}
                />
              </div>
            </>
          )}
        </Space>
      </Card>

      <Card title="SFTP 서버" size="small">
        <Space vertical size="small" style={{ width: '100%' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <div>
              <Text strong>활성화</Text>
              <Text type="secondary" style={{ fontSize: 12, marginLeft: 8 }}>
                (구현 예정)
              </Text>
            </div>
            <Switch
              checked={server.sftpEnabled}
              onChange={(checked: boolean) => updateServerConfig('sftpEnabled', checked)}
            />
          </div>

          {server.sftpEnabled && (
            <>
              <Divider style={{ margin: '6px 0' }} />
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <Text strong>포트</Text>
                <InputNumber
                  size="small"
                  min={1}
                  max={65535}
                  value={server.sftpPort}
                  onChange={(value: number | null) => value && updateServerConfig('sftpPort', value)}
                  style={{ width: 100 }}
                />
              </div>
            </>
          )}
        </Space>
      </Card>
    </Space>
  );
};

export default ServerSettings;
