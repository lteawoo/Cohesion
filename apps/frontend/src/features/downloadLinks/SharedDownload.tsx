import { useEffect, useState } from 'react';
import { Alert, Button, Card, Flex, Spin, Typography, theme } from 'antd';
import { DownloadOutlined } from '@ant-design/icons';
import { useLocation } from 'react-router';
import { useTranslation } from 'react-i18next';
import { formatSize } from '@/features/browse/constants';
import { sharedFileEndpoint, type SharedFile } from './api';

type DownloadState = { status: 'ready'; file: SharedFile } | { status: 'loading' | 'unavailable' | 'error' };

export default function SharedDownload() {
  const { hash } = useLocation();
  return <DownloadPage key={hash} token={hash.slice(1)} />;
}

function DownloadPage({ token }: { token: string }) {
  const { t, i18n } = useTranslation();
  const { token: colors } = theme.useToken();
  const [state, setState] = useState<DownloadState>({ status: token ? 'loading' : 'unavailable' });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    if (!token) return;
    const controller = new AbortController();
    async function load() {
      const response = await fetch(sharedFileEndpoint, {
        headers: { 'X-Download-Link': token }, credentials: 'omit',
        cache: 'no-store', referrerPolicy: 'no-referrer', signal: controller.signal,
      });
      if (response.status === 404 || response.status === 410) {
        if (!controller.signal.aborted) setState({ status: 'unavailable' });
        return;
      }
      if (!response.ok) throw new Error('Could not load shared file');
      const file: SharedFile = await response.json();
      if (!controller.signal.aborted) setState({ status: 'ready', file });
    }
    void load().catch(() => {
      if (!controller.signal.aborted) setState({ status: 'error' });
    });
    return () => controller.abort();
  }, [token, attempt]);

  return <main style={{ minHeight: '100dvh', background: colors.colorBgLayout, padding: 'clamp(24px, 9vh, 96px) 16px', boxSizing: 'border-box' }}>
    <Card style={{ maxWidth: 520, margin: '0 auto' }}>
      <Typography.Title level={1} style={{ fontSize: 28, marginTop: 0, overflowWrap: 'anywhere' }}>
        {state.status === 'ready' ? (state.file.items ? t('downloadLink.sharedCount', { count: state.file.items.length }) : state.file.name) : t('downloadLink.sharedFile')}
      </Typography.Title>
      {state.status === 'loading' && <Spin aria-label={t('downloadLink.loading')} />}
      {state.status === 'unavailable' && <Alert type="warning" showIcon title={t('downloadLink.unavailable')} />}
      {state.status === 'error' && <Alert type="error" title={t('downloadLink.loadError')}
        action={<Button onClick={() => { setState({ status: 'loading' }); setAttempt(n => n + 1); }}>{t('fileRequest.retry')}</Button>} />}
      {state.status === 'ready' && <Flex vertical gap={24}>
        <Typography.Text type="secondary">
          {formatSize(state.file.size)} · {t('downloadLink.expires', { date: new Date(state.file.expires * 1000).toLocaleString(i18n.language) })}
        </Typography.Text>
        <form method="post" action={sharedFileEndpoint} rel="noreferrer">
          <input type="hidden" name="token" value={token} />
          <Button icon={<DownloadOutlined aria-hidden />} type="primary" size="large" htmlType="submit">{t(state.file.items ? 'downloadLink.downloadAll' : 'folderContent.download')}</Button>
        </form>
        {state.file.items && <ul style={{ listStyle: 'none', margin: 0, padding: 0 }}>
          {state.file.items.map(item => <li key={item.id} style={{ paddingBlock: 8, borderBottom: `1px solid ${colors.colorBorderSecondary}` }}>
            <Flex gap={12} align="center" justify="space-between">
              <div style={{ minWidth: 0, overflowWrap: 'anywhere' }}>
                <Typography.Text>{item.name}</Typography.Text><br />
                <Typography.Text type="secondary">{formatSize(item.size)}</Typography.Text>
              </div>
              <form method="post" action={sharedFileEndpoint} rel="noreferrer">
                <input type="hidden" name="token" value={token} />
                <input type="hidden" name="item" value={item.id} />
                <Button htmlType="submit" size="large" icon={<DownloadOutlined aria-hidden />} aria-label={t('downloadLink.downloadItem', { name: item.name })} />
              </form>
            </Flex>
          </li>)}
        </ul>}
      </Flex>}
    </Card>
  </main>;
}
