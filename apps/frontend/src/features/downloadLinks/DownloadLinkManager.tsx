import { useEffect, useRef, useState } from 'react';
import { Alert, App, Button, Divider, Flex, Form, Input, InputNumber, Modal, Popconfirm, Spin, Typography } from 'antd';
import type { InputRef } from 'antd';
import { useTranslation } from 'react-i18next';
import { formatSize } from '@/features/browse/constants';
import { DownloadLinkRequestError, manageDownloadLinks, shareAddress, type DownloadLink } from './api';

interface ManagerProps {
  spaceId: number;
  path?: string;
  onClose: () => void;
}

function errorMessage(error: unknown) {
  if (error instanceof DownloadLinkRequestError) {
    switch (error.status) {
      case 403: return 'downloadLink.forbidden';
      case 409: return 'downloadLink.conflict';
      case 429: return 'downloadLink.busy';
      case 507: return 'downloadLink.noStorage';
    }
  }
  return 'downloadLink.manageError';
}

export default function DownloadLinkManager({ spaceId, path, onClose }: ManagerProps) {
  const { t, i18n } = useTranslation();
  const [days, setDays] = useState<number | null>(7);
  const [links, setLinks] = useState<DownloadLink[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);
  const mutation = useRef<AbortController | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    const params = new URLSearchParams();
    if (path) params.set('path', path);
    void manageDownloadLinks(spaceId, params, { signal: controller.signal })
      .then(response => response.json())
      .then((result: DownloadLink[]) => { if (!controller.signal.aborted) setLinks(result); })
      .catch(err => { if (!controller.signal.aborted) setError(errorMessage(err)); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => { controller.abort(); mutation.current?.abort(); };
  }, [spaceId, path, attempt]);

  async function update(id?: number) {
    if (mutation.current || loading) return;
    const controller = new AbortController();
    mutation.current = controller;
    setBusy(true);
    setError(null);
    try {
      if (id !== undefined) {
        await manageDownloadLinks(spaceId, new URLSearchParams({ id: String(id) }), { method: 'DELETE', signal: controller.signal });
        if (!controller.signal.aborted) setLinks(previous => previous.filter(link => link.id !== id));
      } else if (path && days && days >= 1 && days <= 365 && Number.isInteger(days)) {
        const response = await manageDownloadLinks(spaceId, new URLSearchParams(), {
          method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ path, days }), signal: controller.signal,
        });
        const link: DownloadLink = await response.json();
        if (!controller.signal.aborted) setLinks(previous => [link, ...previous.filter(item => item.id !== link.id)]);
      }
    } catch (err) {
      if (!controller.signal.aborted) setError(errorMessage(err));
    } finally {
      mutation.current = null;
      if (!controller.signal.aborted) setBusy(false);
    }
  }

  return <Modal open onCancel={onClose} footer={null} title={t(path ? 'downloadLink.create' : 'downloadLink.manage')}>
    {path && <Form layout="vertical" onFinish={() => void update()}>
      <Typography.Paragraph strong style={{ overflowWrap: 'anywhere' }}>{path}</Typography.Paragraph>
      <Typography.Paragraph type="secondary">{t('downloadLink.snapshotHint')}</Typography.Paragraph>
      <Form.Item label={t('fileRequest.days')} htmlFor="download-link-days">
        <InputNumber id="download-link-days" min={1} max={365} precision={0} value={days} onChange={setDays} />
      </Form.Item>
      <Button htmlType="submit" type="primary" loading={busy} disabled={loading || !!error || !days || days < 1 || days > 365}>{t('downloadLink.create')}</Button>
    </Form>}
    {loading && <Spin aria-label={t('downloadLink.loading')} />}
    {error && <Alert type="error" showIcon title={t(error)}
      action={<Button disabled={busy} onClick={() => { setError(null); setLoading(true); setAttempt(n => n + 1); }}>{t('fileRequest.retry')}</Button>} />}
    {!path && !loading && !error && links.length === 0 && <Typography.Text type="secondary">{t('downloadLink.empty')}</Typography.Text>}
    {links.map(link => <section key={link.id} aria-label={link.name}>
      <Divider style={{ marginBlock: 16 }} />
      <Flex vertical gap={8}>
        {!path && <Typography.Text strong style={{ overflowWrap: 'anywhere' }}>{link.path}</Typography.Text>}
        <Typography.Text type="secondary">
          {formatSize(link.size)} · {t('downloadLink.expires', { date: new Date(link.expires * 1000).toLocaleString(i18n.language) })}
        </Typography.Text>
        <LinkActions link={link} busy={busy || loading} onRevoke={() => void update(link.id)} />
      </Flex>
    </section>)}
  </Modal>;
}

function LinkActions({ link, busy, onRevoke }: { link: DownloadLink; busy: boolean; onRevoke: () => void }) {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const input = useRef<InputRef>(null);
  const address = shareAddress(link.token);
  async function copy() {
    try {
      await navigator.clipboard.writeText(address);
      void message.success(t('fileRequest.copied'));
    } catch {
      input.current?.focus();
      input.current?.select();
      void message.info(t('fileRequest.copyHint'));
    }
  }
  return <>
    <Input ref={input} value={address} aria-label={t('downloadLink.address')} readOnly onFocus={(event: React.FocusEvent<HTMLInputElement>) => event.currentTarget.select()} />
    <Flex wrap gap={8}>
      <Button onClick={() => void copy()}>{t('fileRequest.copy')}</Button>
      <Popconfirm title={t('downloadLink.confirmClose')} onConfirm={onRevoke} okText={t('downloadLink.close')} cancelText={t('folderContent.cancel')}>
        <Button danger disabled={busy}>{t('downloadLink.close')}</Button>
      </Popconfirm>
    </Flex>
  </>;
}
