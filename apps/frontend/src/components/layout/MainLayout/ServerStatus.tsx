import { App, Popover, theme } from 'antd';
import { useServerStatus } from '@/features/status/hooks/useServerStatus';
import { useUpdateCheck } from '@/features/status/hooks/useUpdateCheck';
import type { ProtocolStatus, UpdateCheckResponse } from '@/features/status/types';
import { useTranslation } from 'react-i18next';

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

function getStatusLabel(status: ProtocolStatus['status'], t: (key: string) => unknown) {
  switch (status) {
    case 'healthy':
      return String(t('serverStatus.status.healthy'));
    case 'unhealthy':
      return String(t('serverStatus.status.unhealthy'));
    case 'unavailable':
      return String(t('serverStatus.status.unavailable'));
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

function formatCheckedAt(checkedAt?: string) {
  if (!checkedAt) {
    return '';
  }

  const parsed = new Date(checkedAt);
  if (Number.isNaN(parsed.getTime())) {
    return '';
  }

  return parsed.toLocaleString();
}

async function copyTextToClipboard(text: string): Promise<boolean> {
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
      return true;
    }
  } catch {
    // continue to fallback
  }

  try {
    const textarea = document.createElement('textarea');
    textarea.value = text;
    textarea.style.position = 'fixed';
    textarea.style.opacity = '0';
    document.body.appendChild(textarea);
    textarea.focus();
    textarea.select();
    const copied = document.execCommand('copy');
    document.body.removeChild(textarea);
    return copied;
  } catch {
    return false;
  }
}

function PopoverContent({
  protocols,
  hosts,
  updateInfo,
}: {
  protocols: Record<string, ProtocolStatus>;
  hosts?: string[];
  updateInfo: UpdateCheckResponse | null;
}) {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  const { message } = App.useApp();
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
        {t('serverStatus.update')}
      </div>
      {updateInfo ? (
        <>
          <div style={{ fontSize: 12, color: token.colorTextSecondary, marginBottom: 2 }}>
            {updateInfo.currentVersion}
            {updateInfo.latestVersion ? ` → ${updateInfo.latestVersion}` : ''}
          </div>
          {updateInfo.updateAvailable ? (
            <a
              href={updateInfo.releaseUrl}
              target="_blank"
              rel="noreferrer"
              style={{ fontSize: 12 }}
            >
              {t('serverStatus.updateAvailable')}
            </a>
          ) : (
            <div style={{ fontSize: 12, color: token.colorTextTertiary }}>
              {updateInfo.error ? t('serverStatus.updateCheckFailed') : t('serverStatus.upToDate')}
            </div>
          )}
          {formatCheckedAt(updateInfo.checkedAt) && (
            <div style={{ fontSize: 11, color: token.colorTextTertiary, marginTop: 2, marginBottom: 10 }}>
              {t('serverStatus.checkedAt', { value: formatCheckedAt(updateInfo.checkedAt) })}
            </div>
          )}
        </>
      ) : (
        <div style={{ fontSize: 12, color: token.colorTextTertiary, marginBottom: 10 }}>
          {t('serverStatus.updateCheckUnavailable')}
        </div>
      )}

      <div style={{ fontSize: 12, color: token.colorTextSecondary, marginBottom: 8 }}>
        {t('serverStatus.hosts')}
      </div>
      {webUrls.map((url) => (
        <div key={url} style={{ padding: '2px 0' }}>
          <button
            type="button"
            className="allow-native-context-menu"
            onClick={async () => {
              const copied = await copyTextToClipboard(url);
              if (copied) {
                message.success(t('serverStatus.hostCopied'));
                return;
              }
              message.error(t('serverStatus.hostCopyFailed'));
            }}
            title={t('serverStatus.copyHostAddress')}
            style={{
              padding: 0,
              border: 0,
              background: 'transparent',
              fontSize: 12,
              color: token.colorTextSecondary,
              cursor: 'pointer',
              textAlign: 'left',
            }}
          >
            {url}
          </button>
        </div>
      ))}
      <div style={{ fontSize: 12, color: token.colorTextSecondary, marginTop: 12, marginBottom: 8 }}>
        {t('serverStatus.protocols')}
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
              {getStatusLabel(proto.status, t)}
            </span>
          </div>
        );
      })}
    </div>
  );
}

export default function ServerStatus() {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  const { status, isServerUp } = useServerStatus();
  const { updateInfo } = useUpdateCheck();

  const dotColor = isServerUp ? token.colorSuccess : token.colorError;

  return (
    <Popover
      content={
        status?.protocols ? (
          <PopoverContent protocols={status.protocols} hosts={status.hosts} updateInfo={updateInfo} />
        ) : (
          <div style={{ fontSize: 12, color: token.colorTextSecondary }}>
            {t('serverStatus.connectionUnavailable')}
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
        <span style={{ fontSize: 13, color: token.colorTextSecondary, lineHeight: 'normal' }}>
          {t('serverStatus.triggerLabel')}
        </span>
        {updateInfo?.updateAvailable && (
          <span
            style={{
              fontSize: 11,
              color: token.colorWarningText,
              background: token.colorWarningBg,
              border: `1px solid ${token.colorWarningBorder}`,
              borderRadius: 999,
              padding: '0 6px',
              lineHeight: '16px',
            }}
          >
            {t('serverStatus.updateBadge')}
          </span>
        )}
      </div>
    </Popover>
  );
}
