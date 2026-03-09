import type { ReactNode } from 'react';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useTransferCenterStore } from '@/stores/transferCenterStore';
import TransferPanel from './TransferPanel';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

vi.mock('@/components/common/BottomSheet', () => ({
  default: ({ open, children }: { open: boolean; children: ReactNode }) => (
    open ? <div data-testid="mock-bottom-sheet">{children}</div> : null
  ),
}));

vi.mock('antd', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('antd');
  return {
    ...actual,
    theme: {
      useToken: () => ({
        token: {
          colorPrimary: '#1677ff',
          colorBgContainer: '#fff',
          colorBgElevated: '#fff',
          colorBorderSecondary: '#d9d9d9',
          boxShadowSecondary: '0 8px 24px rgba(0,0,0,0.12)',
          boxShadowTertiary: '0 12px 32px rgba(0,0,0,0.16)',
        },
      }),
    },
  };
});

describe('TransferPanel', () => {
  beforeEach(() => {
    useTransferCenterStore.getState().reset();
  });

  it('renders a stacked desktop transfer center and clears completed rows', async () => {
    const user = userEvent.setup();
    const cancelUpload = vi.fn();

    useTransferCenterStore.getState().upsertTransfer({
      id: 'completed-1',
      kind: 'download',
      name: 'report.pdf',
      status: 'handed_off',
      updatedAt: 1,
    });
    useTransferCenterStore.getState().upsertTransfer({
      id: 'active-1',
      kind: 'upload',
      name: 'active.bin',
      status: 'uploading',
      loaded: 8,
      total: 16,
      progressPercent: 50,
      updatedAt: 2,
    });

    render(<TransferPanel isMobile={false} onCancelUpload={cancelUpload} />);

    expect(screen.getByTestId('transfer-center-trigger')).toBeTruthy();
    const panel = screen.getByTestId('transfer-center-panel');
    const desktopList = screen.getByTestId('transfer-center-list') as HTMLDivElement;
    expect(desktopList.style.overflowY).toBe('auto');
    expect(desktopList.style.maxHeight).toBe('420px');
    const rowLabels = within(panel).getAllByText(/active\.bin|report\.pdf/);
    expect(rowLabels[0].textContent).toContain('active.bin');
    expect(rowLabels[1].textContent).toContain('report.pdf');
    expect(screen.getByText('fileOperations.transferStatusCompleted')).toBeTruthy();
    expect(screen.getByText('100%')).toBeTruthy();

    await user.click(screen.getByText('fileOperations.cancelTransfer'));
    expect(cancelUpload).toHaveBeenCalledWith('active-1');

    await user.click(screen.getByTestId('transfer-center-clear-completed'));
    expect(screen.queryByText('report.pdf')).toBeNull();
    expect(screen.getByText('active.bin')).toBeTruthy();
  });

  it('allows queued archive rows to be canceled from the stacked panel', async () => {
    const user = userEvent.setup();
    const cancelUpload = vi.fn();

    useTransferCenterStore.getState().upsertTransfer({
      id: 'archive-queued-1',
      kind: 'archive',
      name: 'docs.zip',
      status: 'queued',
      processedItems: 0,
      totalItems: 0,
      processedSourceBytes: 0,
      totalSourceBytes: 0,
      updatedAt: 3,
    });

    render(<TransferPanel isMobile={false} onCancelUpload={cancelUpload} />);

    await user.click(screen.getByText('fileOperations.cancelTransfer'));
    expect(cancelUpload).toHaveBeenCalledWith('archive-queued-1');
  });

  it('renders long transfer names with middle truncation while preserving the extension', () => {
    useTransferCenterStore.getState().upsertTransfer({
      id: 'long-name-1',
      kind: 'download',
      name: 'abcdefghijklmnopqrstuvwxyz1234567890.pdf',
      status: 'handed_off',
      updatedAt: 4,
    });

    render(<TransferPanel isMobile={false} onCancelUpload={vi.fn()} />);

    expect(screen.getByText('abcdefghijklmnopqr...4567890.pdf')).toBeTruthy();
    expect(screen.queryByText('abcdefghijklmnopqrstuvwxyz1234567890.pdf')).toBeNull();
  });

  it('opens the mobile transfer sheet from the floating trigger', async () => {
    const user = userEvent.setup();

    useTransferCenterStore.getState().upsertTransfer({
      id: 'download-1',
      kind: 'download',
      name: 'manual.pdf',
      status: 'handed_off',
    });
    useTransferCenterStore.getState().setOpen(false);

    render(<TransferPanel isMobile onCancelUpload={vi.fn()} />);

    expect(screen.queryByTestId('mock-bottom-sheet')).toBeNull();
    await user.click(screen.getByTestId('transfer-center-trigger'));
    expect(screen.getByTestId('mock-bottom-sheet')).toBeTruthy();
    expect(screen.getByTestId('transfer-center-mobile-sheet')).toBeTruthy();
    const mobileList = screen.getByTestId('transfer-center-list') as HTMLDivElement;
    expect(mobileList.style.overflowY).toBe('auto');
    expect(mobileList.style.maxHeight).toBe('480px');
    expect(screen.getByText('manual.pdf')).toBeTruthy();
    expect(screen.getByText('fileOperations.transferStatusCompleted')).toBeTruthy();
    expect(screen.getByText('100%')).toBeTruthy();
  });
});
