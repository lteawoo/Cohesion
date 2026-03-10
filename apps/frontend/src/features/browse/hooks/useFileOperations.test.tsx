import { act, renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useTransferCenterStore } from '@/stores/transferCenterStore';
import { useFileOperations } from './useFileOperations';

const waitFor = vi.waitFor;

const h = vi.hoisted(() => {
  const apiFetch = vi.fn();
  const apiUpload = vi.fn();
  const message = {
    error: vi.fn(),
    info: vi.fn(),
    success: vi.fn(),
    warning: vi.fn(),
  };
  const modal = {
    confirm: vi.fn(),
  };
  const fetchSpaceContents = vi.fn();
  const invalidateTree = vi.fn();
  const browseState = {
    content: [
      {
        name: 'docs',
        path: '/docs',
        isDir: true,
        size: 0,
        modTime: '2026-03-01T00:00:00.000Z',
      },
      {
        name: 'report.pdf',
        path: '/report.pdf',
        isDir: false,
        size: 128,
        modTime: '2026-03-01T00:00:00.000Z',
      },
    ],
    fetchSpaceContents,
    invalidateTree,
  };

  return {
    apiFetch,
    apiUpload,
    message,
    modal,
    browseState,
    fetchSpaceContents,
    invalidateTree,
  };
});

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, values?: Record<string, string | number>) => {
      if (values?.name) {
        return `${key}:${values.name}`;
      }
      return key;
    },
  }),
}));

vi.mock('antd', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('antd');
  return {
    ...actual,
    App: {
      useApp: () => ({
        message: h.message,
        modal: h.modal,
      }),
    },
  };
});

vi.mock('@/stores/browseStore', () => ({
  useBrowseStore: (selector: (state: typeof h.browseState) => unknown) => selector(h.browseState),
}));

vi.mock('@/api/client', () => ({
  apiFetch: h.apiFetch,
  apiUpload: h.apiUpload,
}));

function jsonResponse(payload: unknown, status = 200): Response {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

function createDeferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  const promise = new Promise<T>((nextResolve) => {
    resolve = nextResolve;
  });
  return { promise, resolve };
}

describe('useFileOperations transfer states', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useRealTimers();
    window.sessionStorage.clear();
    useTransferCenterStore.persist.clearStorage();
    useTransferCenterStore.getState().reset();
    h.browseState.content = [
      {
        name: 'docs',
        path: '/docs',
        isDir: true,
        size: 0,
        modTime: '2026-03-01T00:00:00.000Z',
      },
      {
        name: 'report.pdf',
        path: '/report.pdf',
        isDir: false,
        size: 128,
        modTime: '2026-03-01T00:00:00.000Z',
      },
    ];
  });

  it('tracks upload progress and keeps canceled uploads visible', async () => {
    h.apiUpload.mockImplementation(async (_url: string, _init: RequestInit, options?: { onUploadProgress?: (loaded: number, total: number) => void; signal?: AbortSignal }) => {
      options?.onUploadProgress?.(32, 64);
      await new Promise<never>((_resolve, reject) => {
        options?.signal?.addEventListener(
          'abort',
          () => reject(new DOMException('The operation was aborted.', 'AbortError')),
          { once: true }
        );
      });
      return jsonResponse({});
    });

    const { result } = renderHook(() => useFileOperations('/', { id: 1, name: 'Workspace' } as never));
    act(() => {
      void result.current.handleFileUpload([new File(['x'.repeat(64)], 'large.bin')], '/');
    });

    await waitFor(() => {
      expect(result.current.transfers[0]).toMatchObject({
        kind: 'upload',
        name: 'large.bin',
        status: 'uploading',
        progressPercent: 50,
      });
    });

    act(() => {
      result.current.cancelUpload(result.current.transfers[0].id);
    });

    await waitFor(() => {
      expect(result.current.transfers[0]).toMatchObject({
        kind: 'upload',
        name: 'large.bin',
        status: 'canceled',
        message: 'fileOperations.transferCanceled',
      });
    });
    expect(h.message.info).toHaveBeenCalled();
  });

  it('keeps completed uploads visible in session history', async () => {
    h.apiUpload.mockResolvedValueOnce(jsonResponse({
      filename: 'large.bin',
      status: 'uploaded',
    }));

    const { result } = renderHook(() => useFileOperations('/', { id: 1, name: 'Workspace' } as never));

    await act(async () => {
      await result.current.handleFileUpload([new File(['x'.repeat(64)], 'large.bin')], '/');
    });

    expect(result.current.transfers[0]).toMatchObject({
      kind: 'upload',
      name: 'large.bin',
      status: 'completed',
      progressPercent: 100,
    });
  });

  it('queues excess upload batches and lets queued uploads be canceled before execution', async () => {
    const uploadResponses = [
      createDeferred<Response>(),
      createDeferred<Response>(),
      createDeferred<Response>(),
    ];
    let uploadCallIndex = 0;
    h.apiUpload.mockImplementation(() => uploadResponses[uploadCallIndex++].promise);

    const { result } = renderHook(() => useFileOperations('/', { id: 1, name: 'Workspace' } as never));

    let firstPromise!: Promise<void>;
    let secondPromise!: Promise<void>;
    let thirdPromise!: Promise<void>;

    act(() => {
      firstPromise = result.current.handleFileUpload([new File(['1'], 'first.bin')], '/');
      secondPromise = result.current.handleFileUpload([new File(['2'], 'second.bin')], '/');
      thirdPromise = result.current.handleFileUpload([new File(['3'], 'third.bin')], '/');
    });

    await waitFor(() => {
      expect(h.apiUpload).toHaveBeenCalledTimes(2);
      expect(result.current.transfers.find((transfer) => transfer.name === 'third.bin')).toMatchObject({
        kind: 'upload',
        status: 'queued',
      });
    });

    act(() => {
      const queuedTransfer = result.current.transfers.find((transfer) => transfer.name === 'third.bin');
      if (!queuedTransfer) {
        throw new Error('third upload transfer not found');
      }
      result.current.cancelUpload(queuedTransfer.id);
    });

    await waitFor(() => {
      expect(result.current.transfers.find((transfer) => transfer.name === 'third.bin')).toMatchObject({
        kind: 'upload',
        status: 'canceled',
        message: 'fileOperations.transferCanceled',
      });
    });
    expect(h.apiUpload).toHaveBeenCalledTimes(2);

    await act(async () => {
      uploadResponses[0].resolve(jsonResponse({ filename: 'first.bin', status: 'uploaded' }));
      uploadResponses[1].resolve(jsonResponse({ filename: 'second.bin', status: 'uploaded' }));
      await Promise.all([firstPromise, secondPromise, thirdPromise]);
    });

    expect(h.apiUpload).toHaveBeenCalledTimes(2);
  });

  it('polls archive preparation until browser handoff and keeps the transfer entry visible', async () => {
    const setTimeoutSpy = vi.spyOn(window, 'setTimeout').mockImplementation((handler: TimerHandler) => {
      if (typeof handler === 'function') {
        handler();
      }
      return 0 as unknown as number;
    });
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined);
    h.apiFetch
      .mockResolvedValueOnce(jsonResponse({
        jobId: 'archive-1',
        status: 'queued',
        fileName: 'docs.zip',
        totalItems: 0,
        processedItems: 0,
        totalSourceBytes: 0,
        processedSourceBytes: 0,
      }, 202))
      .mockResolvedValueOnce(jsonResponse({
        jobId: 'archive-1',
        status: 'running',
        fileName: 'docs.zip',
        totalItems: 4,
        processedItems: 2,
        totalSourceBytes: 400,
        processedSourceBytes: 200,
      }))
      .mockResolvedValueOnce(jsonResponse({
        jobId: 'archive-1',
        status: 'ready',
        fileName: 'docs.zip',
        totalItems: 4,
        processedItems: 4,
        totalSourceBytes: 400,
        processedSourceBytes: 400,
        artifactSize: 240,
      }))
      .mockResolvedValueOnce(jsonResponse({
        downloadUrl: '/api/downloads/archive-1',
        fileName: 'docs.zip',
      }));

    const { result } = renderHook(() => useFileOperations('/', { id: 1, name: 'Workspace' } as never));

    await act(async () => {
      await result.current.handleBulkDownload(['/docs']);
    });

    expect(h.apiFetch).toHaveBeenNthCalledWith(
      1,
      '/api/spaces/1/files/archive-downloads',
      expect.objectContaining({ method: 'POST' })
    );
    expect(result.current.transfers[0]).toMatchObject({
      kind: 'archive',
      name: 'docs.zip',
      status: 'handed_off',
      processedItems: 4,
      totalItems: 4,
    });
    expect(clickSpy).toHaveBeenCalledTimes(1);
    expect(h.message.success).toHaveBeenCalledWith('fileOperations.archiveReady:docs.zip');
    clickSpy.mockRestore();
    setTimeoutSpy.mockRestore();
  });

  it('records direct single-file downloads as handed off to the browser', async () => {
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined);
    h.apiFetch.mockResolvedValueOnce(jsonResponse({
      downloadUrl: '/api/downloads/report',
      fileName: 'report.pdf',
    }));

    const { result } = renderHook(() => useFileOperations('/', { id: 1, name: 'Workspace' } as never));

    await act(async () => {
      await result.current.handleBulkDownload(['/report.pdf']);
    });

    expect(result.current.transfers[0]).toMatchObject({
      kind: 'download',
      name: 'report.pdf',
      status: 'handed_off',
    });
    expect(h.apiFetch).toHaveBeenCalledTimes(1);
    expect(clickSpy).toHaveBeenCalledTimes(1);
    clickSpy.mockRestore();
  });

  it('creates a running transfer row while a direct download ticket request is pending', async () => {
    const pendingTicket = createDeferred<Response>();
    h.apiFetch.mockReturnValueOnce(pendingTicket.promise);
    const { result } = renderHook(() => useFileOperations('/', { id: 1, name: 'Workspace' } as never));

    act(() => {
      void result.current.handleBulkDownload(['/report.pdf']);
    });

    await waitFor(() => {
      expect(result.current.transfers[0]).toMatchObject({
        kind: 'download',
        name: 'report.pdf',
        status: 'running',
      });
    });

    await act(async () => {
      pendingTicket.resolve(jsonResponse({
        downloadUrl: '/api/downloads/report',
        fileName: 'report.pdf',
      }));
    });

    await waitFor(() => {
      expect(result.current.transfers[0]).toMatchObject({
        kind: 'download',
        name: 'report.pdf',
        status: 'handed_off',
      });
    });
  });

  it('allows pending direct downloads to be canceled before browser handoff', async () => {
    h.apiFetch.mockImplementationOnce(async (_url: string, init?: RequestInit) => {
      await new Promise<never>((_resolve, reject) => {
        init?.signal?.addEventListener(
          'abort',
          () => reject(new DOMException('The operation was aborted.', 'AbortError')),
          { once: true }
        );
      });
      return jsonResponse({});
    });
    const { result } = renderHook(() => useFileOperations('/', { id: 1, name: 'Workspace' } as never));

    act(() => {
      void result.current.handleBulkDownload(['/report.pdf']);
    });

    await waitFor(() => {
      expect(result.current.transfers[0]).toMatchObject({
        kind: 'download',
        status: 'running',
      });
    });

    act(() => {
      result.current.cancelUpload(result.current.transfers[0].id);
    });

    await waitFor(() => {
      expect(result.current.transfers[0]).toMatchObject({
        kind: 'download',
        name: 'report.pdf',
        status: 'canceled',
        message: 'fileOperations.transferCanceled',
      });
    });
    expect(h.message.error).not.toHaveBeenCalled();
  });

  it('marks direct single-file downloads as failed when the ticket request rejects', async () => {
    h.apiFetch.mockRejectedValueOnce(new Error('network down'));

    const { result } = renderHook(() => useFileOperations('/', { id: 1, name: 'Workspace' } as never));

    await act(async () => {
      await result.current.handleBulkDownload(['/report.pdf']);
    });

    await waitFor(() => {
      expect(result.current.transfers[0]).toMatchObject({
        kind: 'download',
        name: 'report.pdf',
        status: 'failed',
        message: 'network down',
      });
    });
    expect(h.message.error).toHaveBeenCalledWith('network down');
  });

  it('marks persisted uploads as interrupted after reload hydration', async () => {
    vi.spyOn(window.performance, 'getEntriesByType').mockImplementation((entryType: string) => (
      entryType === 'navigation' ? [{ type: 'reload' } as PerformanceNavigationTiming] : []
    ));
    useTransferCenterStore.getState().upsertTransfer({
      id: 'upload-1',
      kind: 'upload',
      name: 'resume.bin',
      status: 'uploading',
      loaded: 32,
      total: 64,
      progressPercent: 50,
      spaceId: 1,
      updatedAt: 1,
    });

    const { result } = renderHook(() => useFileOperations('/', { id: 1, name: 'Workspace' } as never));

    await waitFor(() => {
      expect(result.current.transfers[0]).toMatchObject({
        id: 'upload-1',
        status: 'failed',
        message: 'fileOperations.transferInterruptedOnReload',
      });
    });
  });

  it('marks persisted queued uploads as interrupted after reload hydration', async () => {
    vi.spyOn(window.performance, 'getEntriesByType').mockImplementation((entryType: string) => (
      entryType === 'navigation' ? [{ type: 'reload' } as PerformanceNavigationTiming] : []
    ));
    useTransferCenterStore.getState().upsertTransfer({
      id: 'upload-queued',
      kind: 'upload',
      name: 'queued.bin',
      status: 'queued',
      loaded: 0,
      total: 64,
      progressPercent: 0,
      spaceId: 1,
      updatedAt: 1,
    });

    const { result } = renderHook(() => useFileOperations('/', { id: 1, name: 'Workspace' } as never));

    await waitFor(() => {
      expect(result.current.transfers[0]).toMatchObject({
        id: 'upload-queued',
        status: 'failed',
        message: 'fileOperations.transferInterruptedOnReload',
      });
    });
  });

  it('reattaches persisted archive jobs after reload hydration', async () => {
    vi.spyOn(window.performance, 'getEntriesByType').mockImplementation((entryType: string) => (
      entryType === 'navigation' ? [{ type: 'reload' } as PerformanceNavigationTiming] : []
    ));
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined);

    useTransferCenterStore.getState().upsertTransfer({
      id: 'archive-restore',
      kind: 'archive',
      name: 'docs.zip',
      status: 'running',
      jobId: 'job-restore',
      spaceId: 1,
      processedItems: 1,
      totalItems: 4,
      processedSourceBytes: 100,
      totalSourceBytes: 400,
      updatedAt: 2,
    });

    h.apiFetch
      .mockResolvedValueOnce(jsonResponse({
        jobId: 'job-restore',
        status: 'ready',
        fileName: 'docs.zip',
        totalItems: 4,
        processedItems: 4,
        totalSourceBytes: 400,
        processedSourceBytes: 400,
      }))
      .mockResolvedValueOnce(jsonResponse({
        downloadUrl: '/api/downloads/archive-restore',
        fileName: 'docs.zip',
      }));

    const { result } = renderHook(() => useFileOperations('/', { id: 1, name: 'Workspace' } as never));

    await waitFor(() => {
      expect(result.current.transfers[0]).toMatchObject({
        id: 'archive-restore',
        kind: 'archive',
        status: 'handed_off',
      });
    });
    expect(clickSpy).toHaveBeenCalledTimes(1);
    clickSpy.mockRestore();
  });

  it('surfaces archive preparation failures in transfer state', async () => {
    const setTimeoutSpy = vi.spyOn(window, 'setTimeout').mockImplementation((handler: TimerHandler) => {
      if (typeof handler === 'function') {
        handler();
      }
      return 0 as unknown as number;
    });
    h.apiFetch
      .mockResolvedValueOnce(jsonResponse({
        jobId: 'archive-2',
        status: 'queued',
        fileName: 'docs.zip',
      }, 202))
      .mockResolvedValueOnce(jsonResponse({
        jobId: 'archive-2',
        status: 'failed',
        fileName: 'docs.zip',
        failureReason: 'archive failed',
      }));

    const { result } = renderHook(() => useFileOperations('/', { id: 1, name: 'Workspace' } as never));

    await act(async () => {
      await result.current.handleBulkDownload(['/docs']);
    });

    expect(result.current.transfers[0]).toMatchObject({
      kind: 'archive',
      name: 'docs.zip',
      status: 'failed',
      message: 'archive failed',
    });
    expect(h.message.error).toHaveBeenCalledWith('archive failed');
    setTimeoutSpy.mockRestore();
  });

  it('cancels a running archive job without letting stale polling revive the transfer', async () => {
    const originalSetTimeout = window.setTimeout.bind(window);
    const setTimeoutSpy = vi.spyOn(window, 'setTimeout').mockImplementation((handler: TimerHandler, timeout?: number) => {
      if (timeout === 800 && typeof handler === 'function') {
        handler();
        return 0 as unknown as number;
      }
      return originalSetTimeout(handler, timeout);
    });
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined);
    const statusPoll = createDeferred<Response>();
    const apiCalls: Array<{ url: string; method: string; body?: string }> = [];

    h.apiFetch.mockImplementation((url: string, init?: RequestInit) => {
      apiCalls.push({
        url,
        method: init?.method ?? 'GET',
        body: typeof init?.body === 'string' ? init.body : undefined,
      });

      if (url === '/api/spaces/1/files/archive-downloads' && init?.method === 'POST') {
        return Promise.resolve(jsonResponse({
          jobId: 'archive-cancel-1',
          status: 'queued',
          fileName: 'docs.zip',
          totalItems: 4,
          processedItems: 0,
          totalSourceBytes: 400,
          processedSourceBytes: 0,
        }, 202));
      }

      if (url === '/api/spaces/1/files/archive-downloads?jobId=archive-cancel-1') {
        if (init?.method === 'DELETE') {
          return Promise.resolve(jsonResponse({
            jobId: 'archive-cancel-1',
            status: 'canceled',
            fileName: 'docs.zip',
            totalItems: 4,
            processedItems: 1,
            totalSourceBytes: 400,
            processedSourceBytes: 100,
            failureReason: 'archive canceled',
          }));
        }
        return statusPoll.promise;
      }

      throw new Error(`Unexpected apiFetch call: ${init?.method ?? 'GET'} ${url}`);
    });

    const { result } = renderHook(() => useFileOperations('/', { id: 1, name: 'Workspace' } as never));

    let archivePromise!: Promise<void>;
    act(() => {
      archivePromise = result.current.handleBulkDownload(['/docs']);
    });

    await waitFor(() => {
      expect(result.current.transfers[0]).toMatchObject({
        kind: 'archive',
        name: 'docs.zip',
        status: 'queued',
        jobId: 'archive-cancel-1',
      });
    });

    act(() => {
      result.current.cancelUpload(result.current.transfers[0].id);
    });

    await waitFor(() => {
      expect(result.current.transfers[0]).toMatchObject({
        kind: 'archive',
        status: 'canceled',
        message: 'fileOperations.transferCanceled',
      });
    });

    await act(async () => {
      statusPoll.resolve(jsonResponse({
        jobId: 'archive-cancel-1',
        status: 'ready',
        fileName: 'docs.zip',
        totalItems: 4,
        processedItems: 4,
        totalSourceBytes: 400,
        processedSourceBytes: 400,
      }));
      await archivePromise;
    });

    expect(result.current.transfers[0]).toMatchObject({
      kind: 'archive',
      status: 'canceled',
      message: 'fileOperations.transferCanceled',
    });
    expect(
      apiCalls.some(({ url }) => url === '/api/spaces/1/files/archive-download-ticket')
    ).toBe(false);
    expect(clickSpy).not.toHaveBeenCalled();

    clickSpy.mockRestore();
    setTimeoutSpy.mockRestore();
  });

  it('keeps canceled archive rows intact when a late poll returns an error response', async () => {
    const originalSetTimeout = window.setTimeout.bind(window);
    const setTimeoutSpy = vi.spyOn(window, 'setTimeout').mockImplementation((handler: TimerHandler, timeout?: number) => {
      if (timeout === 800 && typeof handler === 'function') {
        handler();
        return 0 as unknown as number;
      }
      return originalSetTimeout(handler, timeout);
    });
    const lateStatusResponse = createDeferred<Response>();

    h.apiFetch.mockImplementation((url: string, init?: RequestInit) => {
      if (url === '/api/spaces/1/files/archive-downloads' && init?.method === 'POST') {
        return Promise.resolve(jsonResponse({
          jobId: 'archive-cancel-2',
          status: 'queued',
          fileName: 'docs.zip',
        }, 202));
      }

      if (url === '/api/spaces/1/files/archive-downloads?jobId=archive-cancel-2') {
        if (init?.method === 'DELETE') {
          return Promise.resolve(jsonResponse({
            jobId: 'archive-cancel-2',
            status: 'canceled',
            fileName: 'docs.zip',
            failureReason: 'archive canceled',
          }));
        }
        return lateStatusResponse.promise;
      }

      throw new Error(`Unexpected apiFetch call: ${init?.method ?? 'GET'} ${url}`);
    });

    const { result } = renderHook(() => useFileOperations('/', { id: 1, name: 'Workspace' } as never));

    let archivePromise!: Promise<void>;
    act(() => {
      archivePromise = result.current.handleBulkDownload(['/docs']);
    });

    await waitFor(() => {
      expect(result.current.transfers[0]).toMatchObject({
        kind: 'archive',
        status: 'queued',
        jobId: 'archive-cancel-2',
      });
    });

    act(() => {
      result.current.cancelUpload(result.current.transfers[0].id);
    });

    await waitFor(() => {
      expect(result.current.transfers[0]).toMatchObject({
        kind: 'archive',
        status: 'canceled',
        message: 'fileOperations.transferCanceled',
      });
    });

    await act(async () => {
      lateStatusResponse.resolve(new Response(JSON.stringify({ message: 'gone' }), { status: 410 }));
      await archivePromise;
    });

    expect(result.current.transfers[0]).toMatchObject({
      kind: 'archive',
      status: 'canceled',
      message: 'fileOperations.transferCanceled',
    });
    expect(h.message.warning).not.toHaveBeenCalledWith('gone');
    setTimeoutSpy.mockRestore();
  });

  it('retries terminal archive rows with preserved requested paths', async () => {
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined);
    const apiCalls: Array<{ url: string; method: string; body?: string }> = [];
    h.apiFetch.mockImplementation((url: string, init?: RequestInit) => {
      apiCalls.push({
        url,
        method: init?.method ?? 'GET',
        body: typeof init?.body === 'string' ? init.body : undefined,
      });

      if (url === '/api/spaces/1/files/archive-downloads' && init?.method === 'POST') {
        return Promise.resolve(jsonResponse({
          jobId: 'archive-retry-1',
          status: 'ready',
          fileName: 'docs.zip',
          totalItems: 4,
          processedItems: 4,
          totalSourceBytes: 400,
          processedSourceBytes: 400,
          artifactSize: 240,
        }, 202));
      }

      if (url === '/api/spaces/1/files/archive-download-ticket' && init?.method === 'POST') {
        return Promise.resolve(jsonResponse({
          downloadUrl: '/api/downloads/archive-retry-1',
          fileName: 'docs.zip',
        }));
      }

      throw new Error(`Unexpected apiFetch call: ${init?.method ?? 'GET'} ${url}`);
    });

    useTransferCenterStore.getState().upsertTransfer({
      id: 'archive-failed-1',
      kind: 'archive',
      name: 'docs.zip',
      status: 'failed',
      spaceId: 1,
      requestedPaths: ['docs'],
      processedItems: 1,
      totalItems: 4,
      processedSourceBytes: 100,
      totalSourceBytes: 400,
      message: 'archive failed',
      updatedAt: 9,
    });

    const { result } = renderHook(() => useFileOperations('/', { id: 1, name: 'Workspace' } as never));

    act(() => {
      result.current.retryTransfer('archive-failed-1');
    });

    await waitFor(() => {
      expect(result.current.transfers[0]).toMatchObject({
        id: 'archive-failed-1',
        kind: 'archive',
        name: 'docs.zip',
        status: 'handed_off',
        requestedPaths: ['docs'],
        jobId: 'archive-retry-1',
      });
    });

    expect(apiCalls).toContainEqual({
      url: '/api/spaces/1/files/archive-downloads',
      method: 'POST',
      body: JSON.stringify({ paths: ['docs'] }),
    });
    expect(apiCalls).toContainEqual({
      url: '/api/spaces/1/files/archive-download-ticket',
      method: 'POST',
      body: JSON.stringify({ jobId: 'archive-retry-1' }),
    });
    expect(clickSpy).toHaveBeenCalledTimes(1);

    clickSpy.mockRestore();
  });

  it('queues excess archive preparations and lets queued archive work be canceled before execution', async () => {
    h.browseState.content = [
      {
        name: 'docs',
        path: '/docs',
        isDir: true,
        size: 0,
        modTime: '2026-03-01T00:00:00.000Z',
      },
      {
        name: 'media',
        path: '/media',
        isDir: true,
        size: 0,
        modTime: '2026-03-01T00:00:00.000Z',
      },
      {
        name: 'backup',
        path: '/backup',
        isDir: true,
        size: 0,
        modTime: '2026-03-01T00:00:00.000Z',
      },
    ];
    const archiveResponses = [
      createDeferred<Response>(),
      createDeferred<Response>(),
      createDeferred<Response>(),
    ];
    let archiveCallIndex = 0;
    h.apiFetch.mockImplementation((url: string) => {
      if (url === '/api/spaces/1/files/archive-downloads') {
        return archiveResponses[archiveCallIndex++].promise;
      }
      throw new Error(`Unexpected apiFetch call: ${url}`);
    });

    const { result } = renderHook(() => useFileOperations('/', { id: 1, name: 'Workspace' } as never));

    let firstPromise!: Promise<void>;
    let secondPromise!: Promise<void>;
    let thirdPromise!: Promise<void>;

    act(() => {
      firstPromise = result.current.handleBulkDownload(['/docs']);
      secondPromise = result.current.handleBulkDownload(['/media']);
      thirdPromise = result.current.handleBulkDownload(['/backup']);
    });

    await waitFor(() => {
      expect(archiveCallIndex).toBe(2);
      expect(result.current.transfers.find((transfer) => transfer.name === 'backup.zip')).toMatchObject({
        kind: 'archive',
        status: 'queued',
      });
    });

    act(() => {
      const queuedTransfer = result.current.transfers.find((transfer) => transfer.name === 'backup.zip');
      if (!queuedTransfer) {
        throw new Error('queued archive transfer not found');
      }
      result.current.cancelUpload(queuedTransfer.id);
    });

    await waitFor(() => {
      expect(result.current.transfers.find((transfer) => transfer.name === 'backup.zip')).toMatchObject({
        kind: 'archive',
        status: 'canceled',
        message: 'fileOperations.transferCanceled',
      });
    });
    expect(archiveCallIndex).toBe(2);

    await act(async () => {
      archiveResponses[0].resolve(jsonResponse({ message: 'archive failed' }, 500));
      archiveResponses[1].resolve(jsonResponse({ message: 'archive failed' }, 500));
      await Promise.all([firstPromise, secondPromise, thirdPromise]);
    });

    expect(archiveCallIndex).toBe(2);
  });
});
