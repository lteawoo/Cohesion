import React from 'react';

interface HeaderBrandProps {
  text: string;
  color?: string;
  className?: string;
  onClick?: () => void;
  ariaLabel?: string;
  title?: string;
}

const HeaderBrand: React.FC<HeaderBrandProps> = ({ text, color, className, onClick, ariaLabel, title }) => {
  const interactiveProps = onClick
    ? {
        role: 'button' as const,
        tabIndex: 0,
        onClick,
        onKeyDown: (event: React.KeyboardEvent<HTMLSpanElement>) => {
          if (event.key === 'Enter' || event.key === ' ') {
            event.preventDefault();
            onClick();
          }
        },
        'aria-label': ariaLabel,
        title,
        style: {
          ...(color ? { color } : {}),
          cursor: 'pointer',
        },
      }
    : {
        style: color ? { color } : undefined,
      };

  return (
    <span
      className={`layout-header-brand${className ? ` ${className}` : ''}`}
      {...interactiveProps}
    >
      {text}
    </span>
  );
};

export default HeaderBrand;
