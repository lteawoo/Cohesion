import { publicLinkAddress } from '@/api/publicLink';
import { useEffect, useState } from 'react';
import { Alert, App, Button, Form, Input, InputNumber, Modal, Space, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { manageRequest, type FileRequestLink } from './api';

export default function FileRequestManager({ spaceId, folder, onClose }: { spaceId: number; folder: string; onClose: () => void }) {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const [list, setList] = useState<FileRequestLink[]>([]);
  const [days, setDays] = useState<number | null>(7);
  const [limit, setLimit] = useState<number | null>(1024);
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    void manageRequest(spaceId, folder, { signal: controller.signal }).then(r => r.json()).then(items => {
      if (!controller.signal.aborted) setList(items);
    }).catch(() => { if (!controller.signal.aborted) setFailed(true); }).finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [spaceId, folder]);

  async function update(id?: number) {
    setBusy(true);
    setFailed(false);
    try {
      if (id === undefined) {
        const response = await manageRequest(spaceId, folder, {
          method: 'POST', headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ days, byteLimit: (limit ?? 0) * 1024 * 1024 }),
        });
        const link: FileRequestLink = await response.json();
        setList(prev => [link, ...prev.filter(item => item.id !== link.id)]);
      } else {
        await manageRequest(spaceId, folder, { method: 'DELETE' }, id);
        setList(prev => prev.filter(item => item.id !== id));
      }
    } catch { setFailed(true); }
    finally { setBusy(false); }
  }

  return <Modal open onCancel={onClose} footer={null} title={t('fileRequest.manage')}>
    <Form layout="vertical" onFinish={() => void update()}>
      <Space wrap align="start">
        <Form.Item label={t('fileRequest.days')} htmlFor="request-days">
          <InputNumber id="request-days" min={1} max={365} precision={0} value={days} onChange={setDays} />
        </Form.Item>
        <Form.Item label={t('fileRequest.limit')} htmlFor="request-limit">
          <InputNumber id="request-limit" min={1} max={8589934591} precision={0} value={limit} onChange={setLimit} />
        </Form.Item>
      </Space>
      <Form.Item><Button type="primary" htmlType="submit" loading={busy} disabled={loading || !days || !limit}>{t('fileRequest.create')}</Button></Form.Item>
    </Form>
    {failed && <Alert type="error" showIcon title={t('fileRequest.manageError')} />}
    {list.map(link => {
      const url = publicLinkAddress('/request', link.token, link.publicOrigin);
      return <section key={link.id} style={{ marginTop: 16 }}>
        <Typography.Text>{t('fileRequest.details', { date: new Date(link.expires * 1000).toLocaleDateString(), used: (link.received / 1048576).toFixed(1), limit: link.byteLimit / 1048576 })}</Typography.Text>
        <Input aria-label={t('fileRequest.link')} value={url} readOnly onFocus={(event: React.FocusEvent<HTMLInputElement>) => event.currentTarget.select()} />
        <Space style={{ marginTop: 8 }}>
          <Button onClick={async () => {
            try { await navigator.clipboard.writeText(url); void message.success(t('fileRequest.copied')); }
            catch { void message.info(t('fileRequest.copyHint')); }
          }}>{t('fileRequest.copy')}</Button>
          <Button danger disabled={busy} onClick={() => void update(link.id)}>{t('fileRequest.close')}</Button>
        </Space>
      </section>;
    })}
  </Modal>;
}
