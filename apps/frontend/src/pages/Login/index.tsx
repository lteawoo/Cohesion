import { useEffect, useState, type ChangeEvent, type FormEvent } from 'react';
import { App, Button, Card, Input, Space, Typography } from 'antd';
import { LockOutlined, UserOutlined } from '@ant-design/icons';
import { Navigate, useLocation, useNavigate } from 'react-router';
import { useAuth } from '@/features/auth/useAuth';
import { bootstrapAdmin, getSetupStatus } from '@/api/setup';
import { useTranslation } from 'react-i18next';
import '@/assets/css/login.css';

const { Title, Text } = Typography;

const Login = () => {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const navigate = useNavigate();
  const location = useLocation();
  const { user, isLoading, login } = useAuth();
  const [setupChecked, setSetupChecked] = useState(false);
  const [requiresSetup, setRequiresSetup] = useState(false);
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [setupNickname, setSetupNickname] = useState('');
  const [setupPasswordConfirm, setSetupPasswordConfirm] = useState('');

  useEffect(() => {
    if (!isLoading && user) {
      return;
    }

    (async () => {
      try {
        const status = await getSetupStatus();
        setRequiresSetup(status.requiresSetup);
      } catch {
        message.error(t('login.setupStatusFailed'));
      } finally {
        setSetupChecked(true);
      }
    })();
  }, [isLoading, message, t, user]);

  if (!isLoading && user) {
    const state = location.state as { from?: string } | null;
    const to = state?.from ?? '/';
    return <Navigate to={to} replace />;
  }

  if (isLoading || !setupChecked) {
    return (
      <div className="login-page">
        <Card className="login-card">
          <Text type="secondary">{t('login.loading')}</Text>
        </Card>
      </div>
    );
  }

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (username.trim().length < 3) {
      message.error(t('login.usernameMinLength'));
      return;
    }
    if (password.trim().length < 1) {
      message.error(t('login.passwordRequired'));
      return;
    }

    setSubmitting(true);
    try {
      await login(username.trim(), password);
      const state = location.state as { from?: string } | null;
      navigate(state?.from ?? '/', { replace: true });
    } catch (error) {
      message.error(error instanceof Error ? error.message : t('login.loginFailed'));
    } finally {
      setSubmitting(false);
    }
  };

  const handleSetupSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (username.trim().length < 3) {
      message.error(t('login.usernameMinLength'));
      return;
    }
    if (setupPasswordConfirm.trim().length < 8) {
      message.error(t('login.passwordMinLength8'));
      return;
    }
    if (password !== setupPasswordConfirm) {
      message.error(t('login.passwordMismatch'));
      return;
    }

    setSubmitting(true);
    try {
      await bootstrapAdmin({
        username: username.trim(),
        password: password,
        nickname: setupNickname.trim(),
      });
      message.success(t('login.setupSucceeded'));
      setRequiresSetup(false);

      await login(username.trim(), password);
      navigate('/', { replace: true });
    } catch (error) {
      message.error(error instanceof Error ? error.message : t('login.setupFailed'));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="login-page">
      <Card className="login-card">
        <Space orientation="vertical" size="middle" className="login-stack">
          <div>
            <Title level={3} className="login-title">Cohesion</Title>
            <Text type="secondary">
              {requiresSetup ? t('login.setupSubtitle') : t('login.loginSubtitle')}
            </Text>
          </div>

          {requiresSetup ? (
            <form onSubmit={handleSetupSubmit} className="login-form">
              <Space orientation="vertical" size="small" className="login-stack">
                <Input
                  autoComplete="username"
                  prefix={<UserOutlined />}
                  placeholder={t('login.adminUsernamePlaceholder')}
                  value={username}
                  onChange={(event: ChangeEvent<HTMLInputElement>) => setUsername(event.target.value)}
                />
                <Input
                  placeholder={t('login.nicknameOptionalPlaceholder')}
                  value={setupNickname}
                  onChange={(event: ChangeEvent<HTMLInputElement>) => setSetupNickname(event.target.value)}
                />
                <Input.Password
                  autoComplete="new-password"
                  prefix={<LockOutlined />}
                  placeholder={t('login.passwordMinPlaceholder')}
                  value={password}
                  onChange={(event: ChangeEvent<HTMLInputElement>) => setPassword(event.target.value)}
                />
                <Input.Password
                  autoComplete="new-password"
                  prefix={<LockOutlined />}
                  placeholder={t('login.passwordConfirmPlaceholder')}
                  value={setupPasswordConfirm}
                  onChange={(event: ChangeEvent<HTMLInputElement>) => setSetupPasswordConfirm(event.target.value)}
                />
                <Button type="primary" htmlType="submit" loading={submitting} block>
                  {t('login.setupSubmit')}
                </Button>
              </Space>
            </form>
          ) : (
            <form onSubmit={handleSubmit} className="login-form">
              <Space orientation="vertical" size="small" className="login-stack">
                <Input
                  autoComplete="username"
                  prefix={<UserOutlined />}
                  placeholder={t('login.usernamePlaceholder')}
                  value={username}
                  onChange={(event: ChangeEvent<HTMLInputElement>) => setUsername(event.target.value)}
                />
                <Input.Password
                  autoComplete="current-password"
                  prefix={<LockOutlined />}
                  placeholder={t('login.passwordPlaceholder')}
                  value={password}
                  onChange={(event: ChangeEvent<HTMLInputElement>) => setPassword(event.target.value)}
                />
                <Button type="primary" htmlType="submit" loading={submitting} block>
                  {t('login.loginSubmit')}
                </Button>
              </Space>
            </form>
          )}
        </Space>
      </Card>
    </div>
  );
};

export default Login;
