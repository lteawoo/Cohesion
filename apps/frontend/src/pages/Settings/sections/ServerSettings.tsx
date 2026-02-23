import { Card, Switch, InputNumber, Typography, Space, Alert, Divider, Button, App } from 'antd';
import { ReloadOutlined, SaveOutlined } from '@ant-design/icons';
import { useState, useEffect, useCallback, useMemo } from 'react';
import { getConfig, updateConfig, restartServer, waitForReconnect, type Config } from '@/api/config';
import SettingSectionHeader from '../components/SettingSectionHeader';
import SettingRow from '../components/SettingRow';

const { Text } = Typography;

function isValidPort(value: number): boolean {
  return Number.isInteger(value) && value >= 1 && value <= 65535;
}

function parsePortValue(value: string): number | null {
  const trimmed = value.trim();
  if (!/^\d+$/.test(trimmed)) {
    return null;
  }

  const parsed = Number.parseInt(trimmed, 10);
  return isValidPort(parsed) ? parsed : null;
}

function getServerConfigValidationError(server: Config['server']): string | null {
  const webPort = parsePortValue(server.port);
  if (webPort === null) {
    return 'WEB 포트는 1~65535 범위의 숫자여야 합니다.';
  }

  if (server.sftpEnabled) {
    if (!isValidPort(server.sftpPort)) {
      return 'SFTP 포트는 1~65535 범위의 숫자여야 합니다.';
    }
    if (server.sftpPort === webPort) {
      return 'WEB 포트와 SFTP 포트는 서로 달라야 합니다.';
    }
  }

  return null;
}

const ServerSettings = () => {
  const { message, modal } = App.useApp();
  const [config, setConfig] = useState<Config | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [restarting, setRestarting] = useState(false);
  const [hasChanges, setHasChanges] = useState(false);

  // 개발 모드 여부
  const isDev = import.meta.env.DEV;
  const validationError = useMemo(
    () => (config ? getServerConfigValidationError(config.server) : null),
    [config]
  );

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
    if (validationError) {
      message.error(validationError);
      return;
    }

    setSaving(true);
    try {
      await updateConfig(config);
      message.success('설정이 저장되었습니다');
      setHasChanges(false);
    } catch (error) {
      message.error(error instanceof Error ? error.message : '설정 저장에 실패했습니다');
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
        if (hasChanges && validationError) {
          message.error(validationError);
          return;
        }

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
  const serverPortValue = parsePortValue(server.port);

  return (
    <Space vertical size="small" className="settings-section">
      <SettingSectionHeader title="서버 설정" subtitle="서버 및 프로토콜 설정" />

      <Alert
        title="변경 후 재시작 필요"
        type="warning"
        showIcon
        className="settings-alert-compact"
      />

      {validationError && (
        <Alert
          title={validationError}
          type="error"
          showIcon
          className="settings-alert-compact"
        />
      )}

      <Space size="small">
        <Button
          icon={<SaveOutlined />}
          onClick={handleSave}
          loading={saving}
          disabled={!hasChanges || Boolean(validationError)}
          size="small"
          type="primary"
        >
          저장
        </Button>
        <Button
          icon={<ReloadOutlined />}
          onClick={handleRestart}
          loading={restarting}
          disabled={Boolean(validationError)}
          size="small"
        >
          재시작
        </Button>
      </Space>

      <Card title="WEB 서버" size="small">
        <Space vertical size="small" className="settings-stack-full">
          <SettingRow
            left={<Text strong>포트</Text>}
            right={(
              <InputNumber
                size="small"
                min={1}
                max={65535}
                value={serverPortValue}
                onChange={(value: number | null) => {
                  if (value !== null) {
                    updateServerConfig('port', value.toString());
                  }
                }}
                className="settings-port-input"
              />
            )}
          />
        </Space>
      </Card>

      <Card title="WebDAV" size="small">
        <Space vertical size="small" className="settings-stack-full">
          <SettingRow
            left={<Text strong>활성화</Text>}
            right={(
              <Switch
                checked={server.webdavEnabled}
                onChange={(checked: boolean) => updateServerConfig('webdavEnabled', checked)}
              />
            )}
          />

          {server.webdavEnabled && (
            <>
              <Divider className="settings-divider-compact" />
              <SettingRow
                left={<Text strong>경로</Text>}
                right={<Text code>/dav/</Text>}
              />
              <Text type="secondary" className="settings-text-xs">
                HTTP 포트 {server.port} 사용
              </Text>
            </>
          )}
        </Space>
      </Card>

      <Card title="SFTP 서버" size="small">
        <Space vertical size="small" className="settings-stack-full">
          <SettingRow
            left={<Text strong>활성화</Text>}
            right={(
              <Switch
                checked={server.sftpEnabled}
                onChange={(checked: boolean) => updateServerConfig('sftpEnabled', checked)}
              />
            )}
          />

          {server.sftpEnabled && (
            <>
              <Divider className="settings-divider-compact" />
              <SettingRow
                left={<Text strong>포트</Text>}
                right={(
                  <InputNumber
                    size="small"
                    min={1}
                    max={65535}
                    value={server.sftpPort}
                    onChange={(value: number | null) => {
                      if (value !== null) {
                        updateServerConfig('sftpPort', value);
                      }
                    }}
                    className="settings-port-input"
                  />
                )}
              />
            </>
          )}
        </Space>
      </Card>
    </Space>
  );
};

export default ServerSettings;
