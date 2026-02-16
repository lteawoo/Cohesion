import { Navigate, useLocation } from 'react-router';
import { Spin } from 'antd';
import type { ReactElement } from 'react';
import { useAuth } from './useAuth';

const RequireAuth = ({ children }: { children: ReactElement }) => {
  const { user, isLoading } = useAuth();
  const location = useLocation();

  if (isLoading) {
    return (
      <div style={{ display: 'grid', placeItems: 'center', height: '100vh' }}>
        <Spin size="large" />
      </div>
    );
  }

  if (!user) {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />;
  }

  return children;
};

export default RequireAuth;
