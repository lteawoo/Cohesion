import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ChangeEvent, ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import DirectorySetupModal from './DirectorySetupModal';
import { ApiError } from '@/api/error';

const h = vi.hoisted(() => {
  const messageApi = {
    success: vi.fn(),
    error: vi.fn(),
  };
  const storeState = {
    createSpace: vi.fn<(name: string, path: string) => Promise<void>>(),
    validateSpaceRoot: vi.fn<(path: string) => Promise<{ valid: boolean; code: 'valid' | 'not_found' | 'not_directory' | 'permission_denied'; message?: string }>>(),
  };

  return {
    messageApi,
    storeState,
    t: (key: string) => key,
  };
});

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: h.t,
  }),
}));

vi.mock('antd', () => {
  const App = Object.assign(
    ({ children }: { children: ReactNode }) => <div>{children}</div>,
    {
      useApp: () => ({
        message: h.messageApi,
      }),
    }
  );

  return {
    Alert: ({
      message,
      description,
    }: {
      message?: ReactNode;
      description?: ReactNode;
    }) => (
      <div>
        {message ? <div>{message}</div> : null}
        {description ? <div>{description}</div> : null}
      </div>
    ),
    App,
    Input: ({
      value,
      onChange,
      placeholder,
      disabled,
    }: {
      value?: string;
      onChange?: (event: ChangeEvent<HTMLInputElement>) => void;
      placeholder?: string;
      disabled?: boolean;
    }) => (
      <input
        value={value}
        onChange={onChange}
        placeholder={placeholder}
        disabled={disabled}
      />
    ),
    Modal: ({
      open,
      title,
      children,
      onOk,
      onCancel,
      okButtonProps,
      cancelButtonProps,
    }: {
      open: boolean;
      title?: ReactNode;
      children: ReactNode;
      onOk?: () => void;
      onCancel?: () => void;
      okButtonProps?: { disabled?: boolean; loading?: boolean };
      cancelButtonProps?: { disabled?: boolean };
    }) => open ? (
      <div>
        {title ? <h2>{title}</h2> : null}
        {children}
        <button type="button" onClick={onOk} disabled={okButtonProps?.disabled}>
          {okButtonProps?.loading ? 'loading' : 'ok'}
        </button>
        <button type="button" onClick={onCancel} disabled={cancelButtonProps?.disabled}>
          cancel
        </button>
      </div>
    ) : null,
    theme: {
      useToken: () => ({
        token: {
          colorError: '#ff4d4f',
          colorBorder: '#d9d9d9',
        },
      }),
    },
  };
});

vi.mock('../../browse/components/FolderTree', () => ({
  default: ({ onSelect }: { onSelect: (path: string) => void }) => (
    <div>
      <button type="button" onClick={() => onSelect('/valid')}>
        select-valid
      </button>
      <button type="button" onClick={() => onSelect('/denied')}>
        select-denied
      </button>
    </div>
  ),
}));

vi.mock('@/stores/spaceStore', () => ({
  useSpaceStore: (selector: (state: typeof h.storeState) => unknown) => selector(h.storeState),
}));

describe('DirectorySetupModal', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    h.storeState.createSpace.mockResolvedValue();
    h.storeState.validateSpaceRoot.mockImplementation(async (path: string) => ({
      valid: path === '/valid',
      code: path === '/valid' ? 'valid' : 'permission_denied',
    }));
  });

  it('enables creation after a selected root validates successfully', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(<DirectorySetupModal isOpen={true} onClose={onClose} />);

    await user.click(screen.getByRole('button', { name: 'select-valid' }));

    await waitFor(() => {
      expect(h.storeState.validateSpaceRoot).toHaveBeenCalledWith('/valid');
    });
    await waitFor(() => {
      expect(screen.getByDisplayValue('valid')).not.toBeNull();
    });
    expect(screen.getByText('directorySetup.validation.valid')).not.toBeNull();

    const okButton = screen.getByRole('button', { name: 'ok' });
    expect(okButton).toHaveProperty('disabled', false);

    await user.click(okButton);

    await waitFor(() => {
      expect(h.storeState.createSpace).toHaveBeenCalledWith('valid', '/valid');
    });
    expect(h.messageApi.success).toHaveBeenCalledWith('directorySetup.createSuccess');
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('blocks submit and shows permission guidance when the selected root is unreadable', async () => {
    const user = userEvent.setup();
    render(<DirectorySetupModal isOpen={true} onClose={vi.fn()} />);

    await user.click(screen.getByRole('button', { name: 'select-denied' }));

    await waitFor(() => {
      expect(h.storeState.validateSpaceRoot).toHaveBeenCalledWith('/denied');
    });
    expect(screen.getByText('directorySetup.validation.permissionDenied')).not.toBeNull();
    expect(screen.getByText('directorySetup.validation.permissionDeniedHint')).not.toBeNull();
    expect(screen.getByRole('button', { name: 'ok' })).toHaveProperty('disabled', true);
    expect(h.storeState.createSpace).not.toHaveBeenCalled();
  });

  it('keeps the modal open and surfaces create-time revalidation failure', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    h.storeState.createSpace.mockRejectedValue(
      new ApiError('Selected folder is not readable by the server', {
        status: 403,
        code: 'permission_denied',
      })
    );

    render(<DirectorySetupModal isOpen={true} onClose={onClose} />);

    await user.click(screen.getByRole('button', { name: 'select-valid' }));

    const okButton = await screen.findByRole('button', { name: 'ok' });
    await waitFor(() => {
      expect(okButton).toHaveProperty('disabled', false);
    });

    await user.click(okButton);

    await waitFor(() => {
      expect(h.storeState.createSpace).toHaveBeenCalledWith('valid', '/valid');
    });
    expect(h.messageApi.error).toHaveBeenCalledWith('directorySetup.validation.permissionDenied');
    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByText('directorySetup.title')).not.toBeNull();
    expect(screen.getByText('directorySetup.validation.permissionDeniedHint')).not.toBeNull();
  });
});
