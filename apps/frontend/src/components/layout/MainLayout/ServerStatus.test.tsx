import { render } from '@testing-library/react';
import type { ReactNode } from 'react';
import { describe, expect, it, vi } from 'vitest';
import ServerStatus from './ServerStatus';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

vi.mock('@/features/status/hooks/useUpdateCheck', () => ({
  useUpdateCheck: () => ({
    updateInfo: null,
  }),
}));

vi.mock('@/features/status/hooks/useServerStatus', () => ({
  useServerStatus: () => ({
    isServerUp: true,
    isLoading: false,
    refetch: vi.fn(),
    status: {
      hosts: ['localhost:3000'],
      protocols: {
        http: { status: 'healthy', message: 'ok', port: '3000', path: '/' },
        smb: {
          status: 'healthy',
          message: 'ok',
          port: '445',
          endpointMode: 'direct',
          rolloutPhase: 'readonly',
          policySource: 'config',
          bindReady: true,
          runtimeReady: true,
          minVersion: '2.1',
          maxVersion: '3.1.1',
        },
      },
    },
  }),
}));

vi.mock('antd', () => ({
  Popover: ({ content, children }: { content: ReactNode; children: ReactNode }) => (
    <div>
      <div data-testid="popover-content">{content}</div>
      {children}
    </div>
  ),
  App: {
    useApp: () => ({
      message: {
        success: vi.fn(),
        error: vi.fn(),
      },
    }),
  },
  theme: {
    useToken: () => ({
      token: {
        colorSuccess: '#52c41a',
        colorError: '#ff4d4f',
        colorTextTertiary: '#999999',
        colorTextSecondary: '#666666',
        colorWarningText: '#d48806',
        colorWarningBg: '#fffbe6',
        colorWarningBorder: '#ffe58f',
      },
    }),
  },
}));

describe('ServerStatus', () => {
  it('renders protocol status as binary normal/stopped without detail message', () => {
    const view = render(<ServerStatus />);

    expect(view.getByText('SMB')).toBeTruthy();
    expect(view.getAllByText('serverStatus.binaryStatus.normal').length).toBeGreaterThanOrEqual(2);
    expect(view.queryByText('ok')).toBeNull();
    expect(view.queryByText(/direct, phase:readonly, policy:config, bind:ready, runtime:ready, 2.1-3.1.1/)).toBeNull();
  });
});
