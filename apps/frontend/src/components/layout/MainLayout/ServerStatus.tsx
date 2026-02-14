import { Popover, theme } from 'antd';
import { useServerStatus } from '@/features/status/hooks/useServerStatus';
import type { ProtocolStatus } from '@/features/status/types';

const PROTOCOL_LABELS: Record<string, string> = {
  http: 'WEB',
  webdav: 'WebDAV',
  ftp: 'FTP',
};

function StatusDot({ color, size = 8 }: { color: string; size?: number }) {
  return (
    <span
      style={{
        display: 'inline-block',
        width: size,
        height: size,
        borderRadius: '50%',
        backgroundColor: color,
      }}
    />
  );
}

function getStatusColor(status: ProtocolStatus['status']) {
  switch (status) {
    case 'healthy':
      return '#52c41a';
    case 'unhealthy':
      return '#ff4d4f';
    case 'unavailable':
      return '#8c8c8c';
  }
}

function getStatusLabel(status: ProtocolStatus['status']) {
  switch (status) {
    case 'healthy':
      return '정상';
    case 'unhealthy':
      return '오류';
    case 'unavailable':
      return '중지';
  }
}

function PopoverContent({ protocols, hosts }: { protocols: Record<string, ProtocolStatus>; hosts?: string[] }) {
  const { token } = theme.useToken();
  const webUrl = `${window.location.origin}/`;
  const webPort = window.location.port || (window.location.protocol === 'https:' ? '443' : '80');
  const protocol = window.location.protocol;
  const webUrls = (() => {
    if (!hosts || hosts.length === 0) {
      return [webUrl];
    }

    const urls = hosts.map((host) => {
      const portSeparatorIndex = host.lastIndexOf(':');
      const hostname = portSeparatorIndex === -1 ? host : host.slice(0, portSeparatorIndex);
      return `${protocol}//${hostname}:${webPort}/`;
    });

    return [...new Set(urls)];
  })();

  return (
    <div style={{ minWidth: 180 }}>
      <div style={{ fontSize: 12, color: token.colorTextSecondary, marginBottom: 8 }}>
        Protocols
      </div>
      {Object.entries(protocols).map(([key, proto]) => (
        <div
          key={key}
          style={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            padding: '4px 0',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <StatusDot color={getStatusColor(proto.status)} />
            <span style={{ fontSize: 13 }}>{PROTOCOL_LABELS[key] || key}</span>
            {key !== 'http' && proto.port && (
              <span style={{ fontSize: 11, color: token.colorTextTertiary }}>:{proto.port}{proto.path}</span>
            )}
            {key === 'http' && proto.path && (
              <span style={{ fontSize: 11, color: token.colorTextTertiary }}>:{webPort}{proto.path}</span>
            )}
          </div>
          <span style={{ fontSize: 12, color: token.colorTextSecondary }}>
            {getStatusLabel(proto.status)}
          </span>
        </div>
      ))}
      <div style={{ fontSize: 12, color: token.colorTextSecondary, marginTop: 12, marginBottom: 8 }}>
        웹 접근 주소
      </div>
      {webUrls.map((url) => (
        <div key={url} style={{ padding: '2px 0' }}>
          <span style={{ fontSize: 12, color: token.colorTextSecondary }}>{url}</span>
        </div>
      ))}
    </div>
  );
}

export default function ServerStatus() {
  const { token } = theme.useToken();
  const { status, isServerUp } = useServerStatus();

  const dotColor = isServerUp ? '#52c41a' : '#ff4d4f';

  return (
    <Popover
      content={
        status?.protocols ? (
          <PopoverContent protocols={status.protocols} hosts={status.hosts} />
        ) : (
          <div style={{ fontSize: 12, color: token.colorTextSecondary }}>
            서버에 연결할 수 없습니다
          </div>
        )
      }
      trigger="hover"
      placement="bottom"
      arrow={{ pointAtCenter: true }}
    >
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 6,
          cursor: 'pointer',
          lineHeight: 'normal',
          height: 'auto',
          padding: '4px 0',
        }}
      >
        <StatusDot color={dotColor} />
        <span style={{ fontSize: 13, color: token.colorTextSecondary, lineHeight: 'normal' }}>Status</span>
      </div>
    </Popover>
  );
}
