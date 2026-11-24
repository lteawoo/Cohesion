import { useEffect, useRef, useState } from 'react';
import { Alert, Button, Card, Progress, Space, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { newSubmissionKey, sendSubmission, SubmissionError, submitRequest } from './api';

export default function FileDrop() {
  const { t } = useTranslation();
  const token = window.location.hash.slice(1);
  const input = useRef<HTMLInputElement>(null);
  const [title, setTitle] = useState('');
  const [unavailable, setUnavailable] = useState(false);
  const [name, setName] = useState('');
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(false);
  const [error, setError] = useState('');
  const [percent, setPercent] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    void submitRequest(token, undefined, { signal: controller.signal }).then(r => r.json())
      .then(info => setTitle(info.title)).catch(() => { if (!controller.signal.aborted) setUnavailable(true); });
    return () => controller.abort();
  }, [token]);

  async function upload(file: File) {
    setBusy(true); setDone(false); setError(''); setPercent(0); setName(file.name);
    try {
      await sendSubmission(token, newSubmissionKey(), file, offset => setPercent(Math.floor(offset * 100 / Math.max(1,file.size))));
      setPercent(100); setDone(true);
    } catch (err) {
      setError(t(err instanceof SubmissionError && err.status === 409 ? 'fileRequest.capacity' : 'fileRequest.submitError'));
    } finally { setBusy(false); }
  }

  return <main style={{ maxWidth: 520, margin: '0 auto', padding: 'clamp(24px, 9vh, 96px) 16px' }}>
    <Card>
      <Typography.Title level={1} style={{ fontSize: 28, marginTop: 0 }}>{title || t('fileRequest.manage')}</Typography.Title>
      {unavailable ? <Alert type="warning" title={t('fileRequest.unavailable')} /> : <Space orientation="vertical" size={16} style={{ width: '100%' }}>
        <Typography.Text>{t('fileRequest.instructions')}</Typography.Text>
        <input ref={input} type="file" hidden onChange={event => {
          const file = event.target.files?.[0]; event.target.value = '';
          if (file) void upload(file);
        }} />
        <Button type="primary" size="large" disabled={busy || !title} onClick={() => input.current?.click()}>{t('fileRequest.select')}</Button>
        {name && <div aria-live="polite"><Typography.Text>{name}</Typography.Text><Progress percent={percent} status={error ? 'exception' : done ? 'success' : 'active'} /></div>}
        {done && <Alert type="success" showIcon title={t('fileRequest.done')} />}
        {error && <Alert type="error" showIcon title={error} />}
      </Space>}
    </Card>
  </main>;
}
