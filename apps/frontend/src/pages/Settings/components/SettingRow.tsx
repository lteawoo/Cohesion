import React from 'react';

interface SettingRowProps {
  left: React.ReactNode;
  right: React.ReactNode;
  className?: string;
}

const SettingRow: React.FC<SettingRowProps> = ({ left, right, className }) => {
  return (
    <div className={`settings-row-between${className ? ` ${className}` : ''}`}>
      {left}
      {right}
    </div>
  );
};

export default SettingRow;
