import { useState, type ChangeEvent, type FormEvent } from 'react';
import { App, Button, Card, Input, Space, Typography } from 'antd';
import { LockOutlined, UserOutlined } from '@ant-design/icons';
import { Navigate, useLocation, useNavigate } from 'react-router';
import { useAuth } from '@/features/auth/useAuth';
import '@/assets/css/login.css';

const { Title, Text } = Typography;

const Login = () => {
  const { message } = App.useApp();
  const navigate = useNavigate();
  const location = useLocation();
  const { user, isLoading, login } = useAuth();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [submitting, setSubmitting] = useState(false);

  if (!isLoading && user) {
    const state = location.state as { from?: string } | null;
    const to = state?.from ?? '/';
    return <Navigate to={to} replace />;
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

  return (
    <div className="login-page">
      <Card className="login-card">
        <Space orientation="vertical" size="middle" className="login-stack">
          <div>
            <Title level={3} className="login-title">Cohesion</Title>
            <Text type="secondary">계정으로 로그인하세요</Text>
          </div>

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
        </Space>
      </Card>
    </div>
  );
};

export default Login;
