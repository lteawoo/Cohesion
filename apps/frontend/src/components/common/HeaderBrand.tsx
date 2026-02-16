import React from 'react';

interface HeaderBrandProps {
  text: string;
  color?: string;
  className?: string;
}

const HeaderBrand: React.FC<HeaderBrandProps> = ({ text, color, className }) => {
  return (
    <span
      className={`layout-header-brand${className ? ` ${className}` : ''}`}
      style={color ? { color } : undefined}
    >
      {text}
    </span>
  );
};

export default HeaderBrand;
