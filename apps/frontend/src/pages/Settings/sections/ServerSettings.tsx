import { Card, Switch, InputNumber, Typography, Space, Alert, Divider } from 'antd';
import { useSettingsStore } from '@/stores/settingsStore';

const { Title, Text } = Typography;

const ServerSettings = () => {
  // HTTP
  const httpEnabled = useSettingsStore((state) => state.httpEnabled);
  const httpPort = useSettingsStore((state) => state.httpPort);
  const setHttpEnabled = useSettingsStore((state) => state.setHttpEnabled);
  const setHttpPort = useSettingsStore((state) => state.setHttpPort);

  // WebDAV
  const webdavEnabled = useSettingsStore((state) => state.webdavEnabled);
  const webdavPort = useSettingsStore((state) => state.webdavPort);
  const setWebdavEnabled = useSettingsStore((state) => state.setWebdavEnabled);
  const setWebdavPort = useSettingsStore((state) => state.setWebdavPort);

  // FTP
  const ftpEnabled = useSettingsStore((state) => state.ftpEnabled);
  const ftpPort = useSettingsStore((state) => state.ftpPort);
  const setFtpEnabled = useSettingsStore((state) => state.setFtpEnabled);
  const setFtpPort = useSettingsStore((state) => state.setFtpPort);

  // SFTP
  const sftpEnabled = useSettingsStore((state) => state.sftpEnabled);
  const sftpPort = useSettingsStore((state) => state.sftpPort);
  const setSftpEnabled = useSettingsStore((state) => state.setSftpEnabled);
  const setSftpPort = useSettingsStore((state) => state.setSftpPort);

  return (
    <Space direction="vertical" size="large" style={{ width: '100%', maxWidth: 600 }}>
      <div>
        <Title level={3}>서버 설정</Title>
        <Text type="secondary">서버 및 프로토콜 설정</Text>
      </div>

      <Alert
        message="서버 설정 변경 후 재시작 필요"
        description="설정을 변경한 후에는 서버를 재시작해야 적용됩니다."
        type="warning"
        showIcon
      />

      <Card title="HTTP 서버">
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <div>
              <Text strong>HTTP 서버 활성화</Text>
              <div style={{ marginTop: 4 }}>
                <Text type="secondary" style={{ fontSize: 12 }}>
                  웹 인터페이스 및 API 서버
                </Text>
              </div>
            </div>
            <Switch
              checked={httpEnabled}
              onChange={setHttpEnabled}
            />
          </div>

          {httpEnabled && (
            <>
              <Divider style={{ margin: '8px 0' }} />
              <div>
                <Text strong>포트</Text>
                <div style={{ marginTop: 8 }}>
                  <InputNumber
                    min={1}
                    max={65535}
                    value={httpPort}
                    onChange={(value) => value && setHttpPort(value)}
                    style={{ width: 120 }}
                  />
                </div>
                <Text type="secondary" style={{ fontSize: 12, marginTop: 4, display: 'block' }}>
                  HTTP 서버가 사용할 포트 번호 (1-65535)
                </Text>
              </div>
            </>
          )}
        </Space>
      </Card>

      <Card title="WebDAV">
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <div>
              <Text strong>WebDAV 활성화</Text>
              <div style={{ marginTop: 4 }}>
                <Text type="secondary" style={{ fontSize: 12 }}>
                  파일 탐색기/Finder에서 네트워크 드라이브로 접근
                </Text>
              </div>
            </div>
            <Switch
              checked={webdavEnabled}
              onChange={setWebdavEnabled}
              disabled={!httpEnabled}
            />
          </div>

          {webdavEnabled && httpEnabled && (
            <>
              <Divider style={{ margin: '8px 0' }} />
              <div>
                <Text strong>경로</Text>
                <div style={{ marginTop: 8 }}>
                  <Text code>/dav/</Text>
                </div>
                <Text type="secondary" style={{ fontSize: 12, marginTop: 4, display: 'block' }}>
                  WebDAV는 HTTP 서버와 같은 포트를 사용합니다 (포트 {httpPort})
                </Text>
              </div>
            </>
          )}
        </Space>
      </Card>

      <Card title="FTP 서버">
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <div>
              <Text strong>FTP 활성화</Text>
              <div style={{ marginTop: 4 }}>
                <Text type="secondary" style={{ fontSize: 12 }}>
                  FTP 프로토콜로 파일 접근 (구현 예정)
                </Text>
              </div>
            </div>
            <Switch
              checked={ftpEnabled}
              onChange={setFtpEnabled}
            />
          </div>

          {ftpEnabled && (
            <>
              <Divider style={{ margin: '8px 0' }} />
              <div>
                <Text strong>포트</Text>
                <div style={{ marginTop: 8 }}>
                  <InputNumber
                    min={1}
                    max={65535}
                    value={ftpPort}
                    onChange={(value) => value && setFtpPort(value)}
                    style={{ width: 120 }}
                  />
                </div>
                <Text type="secondary" style={{ fontSize: 12, marginTop: 4, display: 'block' }}>
                  FTP 서버가 사용할 포트 번호 (기본: 21)
                </Text>
              </div>
            </>
          )}
        </Space>
      </Card>

      <Card title="SFTP 서버">
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <div>
              <Text strong>SFTP 활성화</Text>
              <div style={{ marginTop: 4 }}>
                <Text type="secondary" style={{ fontSize: 12 }}>
                  SSH 기반 보안 파일 전송 (구현 예정)
                </Text>
              </div>
            </div>
            <Switch
              checked={sftpEnabled}
              onChange={setSftpEnabled}
            />
          </div>

          {sftpEnabled && (
            <>
              <Divider style={{ margin: '8px 0' }} />
              <div>
                <Text strong>포트</Text>
                <div style={{ marginTop: 8 }}>
                  <InputNumber
                    min={1}
                    max={65535}
                    value={sftpPort}
                    onChange={(value) => value && setSftpPort(value)}
                    style={{ width: 120 }}
                  />
                </div>
                <Text type="secondary" style={{ fontSize: 12, marginTop: 4, display: 'block' }}>
                  SFTP 서버가 사용할 포트 번호 (기본: 22)
                </Text>
              </div>
            </>
          )}
        </Space>
      </Card>
    </Space>
  );
};

export default ServerSettings;
