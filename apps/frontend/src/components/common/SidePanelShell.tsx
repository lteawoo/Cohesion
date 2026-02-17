import React from 'react';

interface SidePanelShellProps {
  title: string;
  leftAction?: React.ReactNode;
  rightAction?: React.ReactNode;
  bodyClassName?: string;
  children: React.ReactNode;
}

const SidePanelShell: React.FC<SidePanelShellProps> = ({
  title,
  leftAction,
  rightAction,
  bodyClassName,
  children,
}) => {
  return (
    <>
      <div className="layout-sider-header">
        <div className="layout-sider-header-main">
          {leftAction ? <div className="layout-sider-header-action">{leftAction}</div> : null}
          <span className="layout-sider-title">{title}</span>
        </div>
        {rightAction ? <div className="layout-sider-header-action">{rightAction}</div> : null}
      </div>
      <div className={`layout-sider-body${bodyClassName ? ` ${bodyClassName}` : ''}`}>
        {children}
      </div>
    </>
  );
};

export default SidePanelShell;
