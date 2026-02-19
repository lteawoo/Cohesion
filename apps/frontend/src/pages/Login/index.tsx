import { useEffect, useState, type ChangeEvent, type FormEvent } from 'react';
import { App, Button, Card, Input, Space, Typography } from 'antd';
import { LockOutlined, UserOutlined } from '@ant-design/icons';
import { Navigate, useLocation, useNavigate } from 'react-router';
import { useAuth } from '@/features/auth/useAuth';
import { bootstrapAdmin, getSetupStatus } from '@/api/setup';
import '@/assets/css/login.css';

const { Title, Text } = Typography;

const Login = () => {
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

  if (!isLoading && user) {
    const state = location.state as { from?: string } | null;
    const to = state?.from ?? '/';
    return <Navigate to={to} replace />;
  }

  useEffect(() => {
    (async () => {
      try {
        const status = await getSetupStatus();
        setRequiresSetup(status.requiresSetup);
      } catch {
        message.error('초기 설정 상태를 확인하지 못했습니다');
      } finally {
        setSetupChecked(true);
      }
    })();
  }, [message]);

  if (isLoading || !setupChecked) {
    return (
      <div className="login-page">
        <Card className="login-card">
          <Text type="secondary">로딩 중...</Text>
        </Card>
      </div>
    );
  }

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (username.trim().length < 3) {
      message.error('아이디는 3자 이상이어야 합니다');
      return;
    }
    if (password.trim().length < 1) {
      message.error('비밀번호를 입력하세요');
      return;
    }

    setSubmitting(true);
    try {
      await login(username.trim(), password);
      const state = location.state as { from?: string } | null;
      navigate(state?.from ?? '/', { replace: true });
    } catch (error) {
      message.error(error instanceof Error ? error.message : '로그인에 실패했습니다');
    } finally {
      setSubmitting(false);
    }
  };

  const handleSetupSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (username.trim().length < 3) {
      message.error('아이디는 3자 이상이어야 합니다');
      return;
    }
    if (setupPasswordConfirm.trim().length < 8) {
      message.error('비밀번호는 8자 이상이어야 합니다');
      return;
    }
    if (password !== setupPasswordConfirm) {
      message.error('비밀번호 확인이 일치하지 않습니다');
      return;
    }

    setSubmitting(true);
    try {
      await bootstrapAdmin({
        username: username.trim(),
        password: password,
        nickname: setupNickname.trim(),
      });
      message.success('초기 관리자 계정이 생성되었습니다');
      setRequiresSetup(false);

      await login(username.trim(), password);
      navigate('/', { replace: true });
    } catch (error) {
      message.error(error instanceof Error ? error.message : '초기 설정에 실패했습니다');
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
              {requiresSetup ? '초기 관리자 계정을 생성하세요' : '계정으로 로그인하세요'}
            </Text>
          </div>

          {requiresSetup ? (
            <form onSubmit={handleSetupSubmit} className="login-form">
              <Space orientation="vertical" size="small" className="login-stack">
                <Input
                  autoComplete="username"
                  prefix={<UserOutlined />}
                  placeholder="관리자 아이디"
                  value={username}
                  onChange={(event: ChangeEvent<HTMLInputElement>) => setUsername(event.target.value)}
                />
                <Input
                  placeholder="닉네임 (선택)"
                  value={setupNickname}
                  onChange={(event: ChangeEvent<HTMLInputElement>) => setSetupNickname(event.target.value)}
                />
                <Input.Password
                  autoComplete="new-password"
                  prefix={<LockOutlined />}
                  placeholder="비밀번호 (8자 이상)"
                  value={password}
                  onChange={(event: ChangeEvent<HTMLInputElement>) => setPassword(event.target.value)}
                />
                <Input.Password
                  autoComplete="new-password"
                  prefix={<LockOutlined />}
                  placeholder="비밀번호 확인"
                  value={setupPasswordConfirm}
                  onChange={(event: ChangeEvent<HTMLInputElement>) => setSetupPasswordConfirm(event.target.value)}
                />
                <Button type="primary" htmlType="submit" loading={submitting} block>
                  초기 설정 완료
                </Button>
              </Space>
            </form>
          ) : (
            <form onSubmit={handleSubmit} className="login-form">
              <Space orientation="vertical" size="small" className="login-stack">
                <Input
                  autoComplete="username"
                  prefix={<UserOutlined />}
                  placeholder="아이디"
                  value={username}
                  onChange={(event: ChangeEvent<HTMLInputElement>) => setUsername(event.target.value)}
                />
                <Input.Password
                  autoComplete="current-password"
                  prefix={<LockOutlined />}
                  placeholder="비밀번호"
                  value={password}
                  onChange={(event: ChangeEvent<HTMLInputElement>) => setPassword(event.target.value)}
                />
                <Button type="primary" htmlType="submit" loading={submitting} block>
                  로그인
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
