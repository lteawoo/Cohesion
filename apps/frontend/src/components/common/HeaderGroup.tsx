import React from 'react';

type HeaderGroupAlign = 'start' | 'end';

interface HeaderGroupProps {
  align?: HeaderGroupAlign;
  className?: string;
  children: React.ReactNode;
}

const HeaderGroup: React.FC<HeaderGroupProps> = ({ align = 'start', className, children }) => {
  const baseClass = align === 'start' ? 'layout-header-start' : 'layout-header-end';
  return <div className={`${baseClass}${className ? ` ${className}` : ''}`}>{children}</div>;
};

export default HeaderGroup;
