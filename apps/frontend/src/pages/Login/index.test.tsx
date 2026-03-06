import { render } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import Login from './index';

const h = vi.hoisted(() => {
  const navigate = vi.fn();
  const login = vi.fn<(username: string, password: string) => Promise<void>>();
  const getSetupStatus = vi.fn<() => Promise<{ requiresSetup: boolean }>>();
  const bootstrapAdmin = vi.fn<
    (payload: { username: string; password: string; nickname: string }) => Promise<void>
  >();
  const messageApi = {
    success: vi.fn(),
    error: vi.fn(),
  };
  const authState = {
    user: null as { username: string } | null,
    isLoading: false,
  };
  const locationState = {
    state: null as { from?: string } | null,
  };

  return {
    navigate,
    login,
    getSetupStatus,
    bootstrapAdmin,
    messageApi,
    authState,
    locationState,
  };
});

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

vi.mock('react-router', () => ({
  Navigate: ({ to }: { to: string }) => <div data-testid="navigate" data-to={to} />,
  useNavigate: () => h.navigate,
  useLocation: () => h.locationState,
}));

vi.mock('@/features/auth/useAuth', () => ({
  useAuth: () => ({
    user: h.authState.user,
    isLoading: h.authState.isLoading,
    login: h.login,
  }),
}));

vi.mock('@/api/setup', () => ({
  getSetupStatus: h.getSetupStatus,
  bootstrapAdmin: h.bootstrapAdmin,
}));

vi.mock('@ant-design/icons', () => ({
  LockOutlined: () => null,
  UserOutlined: () => null,
}));

vi.mock('antd', () => {
  const Input = ({
    value,
    onChange,
    placeholder,
  }: InputHTMLAttributes<HTMLInputElement> & {
    prefix?: ReactNode;
  }) => (
    <input
      value={typeof value === 'string' ? value : ''}
      onChange={(event) => onChange?.(event)}
      placeholder={placeholder}
    />
  );

  Input.Password = Input;

  return {
    App: {
      useApp: () => ({
        message: h.messageApi,
      }),
    },
    Button: ({
      children,
      htmlType,
      loading,
      block,
      ...props
    }: ButtonHTMLAttributes<HTMLButtonElement> & {
      htmlType?: 'button' | 'submit' | 'reset';
      loading?: boolean;
      block?: boolean;
      type?: 'primary' | 'default' | 'text';
    }) => (
      <button type={htmlType ?? 'button'} disabled={loading || props.disabled} {...props}>
        {children}
      </button>
    ),
    Card: ({ children }: { children: ReactNode; className?: string }) => <div>{children}</div>,
    Input,
    Space: ({ children }: { children: ReactNode; orientation?: string; size?: string; className?: string }) => (
      <div>{children}</div>
    ),
    Typography: {
      Title: ({ children }: { children: ReactNode; level?: number; className?: string }) => <h1>{children}</h1>,
      Text: ({ children }: { children: ReactNode; type?: string }) => <span>{children}</span>,
    },
  };
});

describe('Login', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    h.authState.user = null;
    h.authState.isLoading = false;
    h.locationState.state = null;
    h.getSetupStatus.mockResolvedValue({ requiresSetup: false });
    h.bootstrapAdmin.mockResolvedValue();
    h.login.mockResolvedValue();
  });

  it('renders a loading state while auth is still loading', () => {
    h.authState.isLoading = true;

    const view = render(<Login />);

    expect(view.getByText('login.loading')).toBeTruthy();
  });

  it('redirects authenticated users to the requested path', () => {
    h.authState.user = { username: 'admin' };
    h.locationState.state = { from: '/settings' };

    const view = render(<Login />);

    expect(view.getByTestId('navigate').getAttribute('data-to')).toBe('/settings');
    expect(h.getSetupStatus).not.toHaveBeenCalled();
  });

  it('submits valid credentials and navigates to the requested path', async () => {
    h.locationState.state = { from: '/search' };
    const user = userEvent.setup();
    const view = render(<Login />);

    await user.type(await view.findByPlaceholderText('login.usernamePlaceholder'), 'admin');
    await user.type(view.getByPlaceholderText('login.passwordPlaceholder'), 'admin1234');
    await user.click(view.getByRole('button', { name: 'login.loginSubmit' }));

    await vi.waitFor(() => {
      expect(h.login).toHaveBeenCalledWith('admin', 'admin1234');
      expect(h.navigate).toHaveBeenCalledWith('/search', { replace: true });
    });
  });

  it('renders the setup flow and bootstraps the first admin', async () => {
    h.getSetupStatus.mockResolvedValue({ requiresSetup: true });
    const user = userEvent.setup();
    const view = render(<Login />);

    await user.type(await view.findByPlaceholderText('login.adminUsernamePlaceholder'), 'admin');
    await user.type(view.getByPlaceholderText('login.nicknameOptionalPlaceholder'), 'Administrator');
    await user.type(view.getByPlaceholderText('login.passwordMinPlaceholder'), 'admin1234');
    await user.type(view.getByPlaceholderText('login.passwordConfirmPlaceholder'), 'admin1234');
    await user.click(view.getByRole('button', { name: 'login.setupSubmit' }));

    await vi.waitFor(() => {
      expect(h.bootstrapAdmin).toHaveBeenCalledWith({
        username: 'admin',
        password: 'admin1234',
        nickname: 'Administrator',
      });
      expect(h.login).toHaveBeenCalledWith('admin', 'admin1234');
      expect(h.navigate).toHaveBeenCalledWith('/', { replace: true });
    });
    expect(h.messageApi.success).toHaveBeenCalledWith('login.setupSucceeded');
  });

  it('rejects invalid login submission before calling the API', async () => {
    const user = userEvent.setup();
    const view = render(<Login />);

    await user.type(await view.findByPlaceholderText('login.usernamePlaceholder'), 'ab');
    await user.click(view.getByRole('button', { name: 'login.loginSubmit' }));

    expect(h.login).not.toHaveBeenCalled();
    expect(h.messageApi.error).toHaveBeenCalledWith('login.usernameMinLength');
  });
});
