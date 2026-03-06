import { App } from 'antd';
import { render } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import ProfileSettings from './ProfileSettings';

const mockNavigate = vi.fn();
const mockUpdateProfile = vi.fn();
const mockRefreshSession = vi.fn();
const mockLogout = vi.fn();
const mockUseAuth = vi.fn();

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

vi.mock('@/features/auth/useAuth', () => ({
  useAuth: () => mockUseAuth(),
}));

vi.mock('@/api/auth', () => ({
  updateProfile: (...args: unknown[]) => mockUpdateProfile(...args),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

function renderProfile() {
  return render(
    <App>
      <ProfileSettings />
    </App>,
  );
}

describe('ProfileSettings', () => {
  beforeEach(() => {
    mockNavigate.mockReset();
    mockUpdateProfile.mockReset();
    mockRefreshSession.mockReset();
    mockLogout.mockReset();
    mockUseAuth.mockReset();

    mockRefreshSession.mockResolvedValue(undefined);
    mockLogout.mockResolvedValue(undefined);
  });

  it('saves nickname and password changes then refreshes the session', async () => {
    const user = userEvent.setup();
    mockUseAuth.mockReturnValue({
      user: {
        id: 7,
        username: 'member',
        nickname: 'Member Tester',
        role: 'user',
        permissions: ['profile.read', 'profile.write'],
      },
      logout: mockLogout,
      refreshSession: mockRefreshSession,
    });
    mockUpdateProfile.mockResolvedValue(undefined);

    const view = renderProfile();

    const nicknameInput = view.getByPlaceholderText('profileSettings.nicknamePlaceholder');
    await user.clear(nicknameInput);
    await user.type(nicknameInput, 'Updated Member');

    await user.click(view.getByRole('button', { name: 'profileSettings.save' }));

    await vi.waitFor(() => {
      expect(mockUpdateProfile).toHaveBeenCalledWith({
        nickname: 'Updated Member',
      });
    });
    await vi.waitFor(() => {
      expect(mockRefreshSession).toHaveBeenCalledTimes(1);
    });
  });

  it('changes password through the password modal', async () => {
    const user = userEvent.setup();
    mockUseAuth.mockReturnValue({
      user: {
        id: 7,
        username: 'member',
        nickname: 'Member Tester',
        role: 'user',
        permissions: ['profile.read', 'profile.write'],
      },
      logout: mockLogout,
      refreshSession: mockRefreshSession,
    });
    mockUpdateProfile.mockResolvedValue(undefined);

    const view = renderProfile();

    await user.click(view.getByRole('button', { name: 'profileSettings.changePasswordAction' }));
    await user.type(view.getByPlaceholderText('profileSettings.currentPasswordPlaceholder'), 'member-test-password');
    await user.type(view.getByPlaceholderText('profileSettings.newPasswordPlaceholder'), 'member-password-updated');
    await user.type(view.getByPlaceholderText('profileSettings.confirmPasswordPlaceholder'), 'member-password-updated');
    await user.click(view.getAllByRole('button', { name: 'profileSettings.changePasswordAction' }).at(-1)!);

    await vi.waitFor(() => {
      expect(mockUpdateProfile).toHaveBeenCalledWith({
        currentPassword: 'member-test-password',
        newPassword: 'member-password-updated',
      });
    });
    await vi.waitFor(() => {
      expect(mockRefreshSession).toHaveBeenCalledTimes(1);
    });
  });

  it('blocks password submission when current password is missing', async () => {
    const user = userEvent.setup();
    mockUseAuth.mockReturnValue({
      user: {
        id: 7,
        username: 'member',
        nickname: 'Member Tester',
        role: 'user',
        permissions: ['profile.read', 'profile.write'],
      },
      logout: mockLogout,
      refreshSession: mockRefreshSession,
    });

    const view = renderProfile();

    await user.click(view.getByRole('button', { name: 'profileSettings.changePasswordAction' }));
    await user.type(view.getByPlaceholderText('profileSettings.newPasswordPlaceholder'), 'member-password-updated');
    await user.type(view.getByPlaceholderText('profileSettings.confirmPasswordPlaceholder'), 'member-password-updated');
    await user.click(view.getAllByRole('button', { name: 'profileSettings.changePasswordAction' }).at(-1)!);

    await vi.waitFor(() => {
      expect(mockUpdateProfile).not.toHaveBeenCalled();
    });
    expect(mockRefreshSession).not.toHaveBeenCalled();
  });

  it('renders read-only profile controls without save action when profile.write is missing', () => {
    mockUseAuth.mockReturnValue({
      user: {
        id: 9,
        username: 'viewer',
        nickname: 'Viewer',
        role: 'user',
        permissions: ['profile.read'],
      },
      logout: mockLogout,
      refreshSession: mockRefreshSession,
    });

    const view = renderProfile();

    expect(view.queryByRole('button', { name: 'profileSettings.save' })).toBeNull();
    expect(view.queryByPlaceholderText('profileSettings.currentPasswordPlaceholder')).toBeNull();
    expect(view.getByPlaceholderText('profileSettings.nicknamePlaceholder').hasAttribute('disabled')).toBe(true);
  });
});
