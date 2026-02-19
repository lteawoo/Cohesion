import { Popover, theme } from 'antd';
import { useServerStatus } from '@/features/status/hooks/useServerStatus';
import type { ProtocolStatus } from '@/features/status/types';

const PROTOCOL_LABELS: Record<string, string> = {
  http: 'WEB',
  webdav: 'WebDAV',
  sftp: 'SFTP',
};
const PROTOCOL_ORDER = ['http', 'webdav', 'sftp'] as const;

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

function getStatusColor(
  status: ProtocolStatus['status'],
  colors: {
    healthy: string;
    unhealthy: string;
    unavailable: string;
  }
) {
  switch (status) {
    case 'healthy':
      return colors.healthy;
    case 'unhealthy':
      return colors.unhealthy;
    case 'unavailable':
      return colors.unavailable;
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

function normalizeProtocolPath(path?: string) {
  if (!path) {
    return '';
  }

  const trimmed = path.trim();
  if (trimmed === '' || trimmed === '/') {
    return '';
  }

  return trimmed.replace(/\/+$/, '');
}

function PopoverContent({ protocols, hosts }: { protocols: Record<string, ProtocolStatus>; hosts?: string[] }) {
  const { token } = theme.useToken();
  const orderedProtocolEntries = [
    ...PROTOCOL_ORDER.filter((key) => protocols[key] !== undefined).map((key) => [key, protocols[key]] as const),
    ...Object.entries(protocols).filter(([key]) => !PROTOCOL_ORDER.includes(key as (typeof PROTOCOL_ORDER)[number])),
  ];
  const statusColors = {
    healthy: token.colorSuccess,
    unhealthy: token.colorError,
    unavailable: token.colorTextTertiary,
  };
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
        Hosts
      </div>
      {webUrls.map((url) => (
        <div key={url} style={{ padding: '2px 0' }}>
          <span style={{ fontSize: 12, color: token.colorTextSecondary }}>{url}</span>
        </div>
      ))}
      <div style={{ fontSize: 12, color: token.colorTextSecondary, marginTop: 12, marginBottom: 8 }}>
        Protocols
      </div>
      {orderedProtocolEntries.map(([key, proto]) => {
        const displayPort = key === 'http' ? webPort : proto.port;
        const displayPath = normalizeProtocolPath(proto.path);

        return (
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
              <StatusDot color={getStatusColor(proto.status, statusColors)} />
              <span style={{ fontSize: 13 }}>{PROTOCOL_LABELS[key] || key}</span>
              {displayPort && (
                <span style={{ fontSize: 11, color: token.colorTextTertiary }}>:{displayPort}{displayPath}</span>
              )}
            </div>
            <span style={{ fontSize: 12, color: token.colorTextSecondary }}>
              {getStatusLabel(proto.status)}
            </span>
          </div>
        );
      })}
    </div>
  );
}

export default function ServerStatus() {
  const { token } = theme.useToken();
  const { status, isServerUp } = useServerStatus();

  const dotColor = isServerUp ? token.colorSuccess : token.colorError;

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
