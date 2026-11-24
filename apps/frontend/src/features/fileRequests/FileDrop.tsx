import { useEffect, useRef, useState } from 'react';
import { Alert, Button, Card, Progress, Space, Typography, theme } from 'antd';
import { useTranslation } from 'react-i18next';
import { useLocation } from 'react-router';
import { sendSubmission, SubmissionError, submitRequest } from './api';
import { matchesSubmission, readSubmission, rememberSubmission, startSubmission, type PendingSubmission } from './pendingSubmission';

export default function FileDrop() {
  const { hash } = useLocation();
  return <SubmissionForm key={hash} token={hash.slice(1)} />;
}

function SubmissionForm({ token }: { token: string }) {
  const { t } = useTranslation();
  const { token: colors } = theme.useToken();
  const input = useRef<HTMLInputElement>(null);
  const transfer = useRef<AbortController | null>(null);
  const [pending, setPending] = useState(() => readSubmission(token));
  const [title, setTitle] = useState('');
  const [availability, setAvailability] = useState<'loading' | 'open' | 'closed' | 'error'>('loading');
  const [reload, setReload] = useState(0);
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState('');
  const [error, setError] = useState('');
  const [offset, setOffset] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    const signal = controller.signal;
    async function load() {
      const info = await (await submitRequest(token, undefined, { signal })).json();
      if (signal.aborted) return;
      setTitle(info.title);
      const previous = readSubmission(token);
      if (previous) {
        try {
          const state = await (await submitRequest(token, previous.key, { signal })).json();
          if (signal.aborted) return;
          if (state.status === 'uploaded') {
            rememberSubmission(token, null);
            setPending(null);
            setDone(previous.name);
          } else {
            setOffset(state.offset);
          }
        } catch (err) {
          // The first POST may never have reached the server. Reselecting reuses its key.
          if (!(err instanceof SubmissionError && err.status === 404)) throw err;
        }
      }
      if (!signal.aborted) setAvailability('open');
    }
    void load().catch(err => {
      if (!signal.aborted) setAvailability(err instanceof SubmissionError && err.status === 404 ? 'closed' : 'error');
    });
    return () => { controller.abort(); transfer.current?.abort(); };
  }, [token, reload]);

  function save(value: PendingSubmission | null) {
    rememberSubmission(token, value);
    setPending(value);
  }

  async function upload(file: File) {
    if (transfer.current) return;
    if (pending && !matchesSubmission(file, pending)) {
      setError(t('fileRequest.wrongFile'));
      return;
    }
    const submission = pending ?? startSubmission(file);
    if (!pending) setOffset(0);
    const controller = new AbortController();
    transfer.current = controller;
    save(submission);
    setError('');
    setDone('');
    setBusy(true);
    try {
      await sendSubmission(token, submission.key, file, setOffset, controller.signal);
      if (!controller.signal.aborted) {
        save(null);
        setDone(file.name);
      }
    } catch (err) {
      if (!controller.signal.aborted) {
        if (err instanceof SubmissionError && err.status === 404) setAvailability('closed');
        else if (err instanceof SubmissionError && (err.status === 507 || err.message === 'Request capacity unavailable')) setError(t('fileRequest.capacity'));
        else setError(t('fileRequest.submitError'));
      }
    } finally {
      transfer.current = null;
      if (!controller.signal.aborted) setBusy(false);
    }
  }

  async function discard() {
    if (!pending || transfer.current) return;
    const controller = new AbortController();
    transfer.current = controller;
    setBusy(true);
    try {
      const response = await submitRequest(token, pending.key, { method: 'DELETE', signal: controller.signal });
      if (response.status === 200 && (await response.json()).status === 'uploaded') setDone(pending.name);
    } catch (err) {
      if (controller.signal.aborted) return;
      if (!(err instanceof SubmissionError && err.status === 404)) { setError(t('fileRequest.cancelError')); return; }
    } finally {
      transfer.current = null;
      if (!controller.signal.aborted) setBusy(false);
    }
    setOffset(0);
    setError('');
    save(null);
  }

  const filename = pending?.name || done;
  const percent = done ? 100 : Math.floor(offset * 100 / Math.max(1, pending?.size ?? 0));
  return <main style={{ minHeight: '100dvh', background: colors.colorBgLayout, padding: 'clamp(24px, 9vh, 96px) 16px', boxSizing: 'border-box' }}>
    <Card style={{ maxWidth: 520, margin: '0 auto' }}>
      <Typography.Title level={1} style={{ fontSize: 28, marginTop: 0, overflowWrap: 'anywhere' }}>{title || t('fileRequest.manage')}</Typography.Title>
      {availability === 'closed' ? <Alert type="warning" title={t('fileRequest.unavailable')} />
        : availability === 'error' ? <Alert type="error" title={t('fileRequest.loadError')} action={<Button onClick={() => { setAvailability('loading'); setReload(n => n + 1); }}>{t('fileRequest.retry')}</Button>} />
        : <Space orientation="vertical" size={16} style={{ width: '100%' }}>
          <Typography.Text>{t(pending && !busy ? 'fileRequest.resumeHint' : 'fileRequest.instructions')}</Typography.Text>
          <input ref={input} type="file" hidden onChange={event => {
            const file = event.target.files?.[0];
            event.target.value = '';
            if (file) void upload(file);
          }} />
          <Space wrap>
            <Button type="primary" size="large" loading={busy} disabled={availability !== 'open'} onClick={() => input.current?.click()}>
              {t(pending ? 'fileRequest.resume' : 'fileRequest.select')}
            </Button>
            {pending && !busy && availability === 'open' && <Button onClick={() => void discard()}>{t('fileRequest.discard')}</Button>}
          </Space>
          {filename && <div aria-live="polite"><Typography.Text style={{ overflowWrap: 'anywhere' }}>{filename}</Typography.Text><Progress percent={percent} status={done ? 'success' : busy ? 'active' : 'normal'} /></div>}
          {done && <Alert type="success" showIcon title={t('fileRequest.done')} />}
          {error && <Alert type="error" showIcon title={error} />}
        </Space>}
    </Card>
  </main>;
}
