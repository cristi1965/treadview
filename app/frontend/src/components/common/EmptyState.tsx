import React from 'react';

interface EmptyStateProps {
  title: string;
  description?: React.ReactNode;
  icon?: React.ReactNode;
  action?: {
    label: string;
    onClick: () => void;
  };
}

export const EmptyState: React.FC<EmptyStateProps> = ({
  title,
  description,
  icon,
  action
}) => {
  return (
    <div className="rounded-xl border-2 border-dashed border-line bg-surface p-16 text-center">
      {icon && (
        <div className="mb-4 flex justify-center text-faint">
          {icon}
        </div>
      )}
      
      <h3 className="mb-3 text-lg font-semibold text-muted">
        {title}
      </h3>
      
      {description && (
        <p className="mb-6 text-sm text-faint leading-relaxed max-w-md mx-auto">
          {description}
        </p>
      )}
      
      {action && (
        <button
          onClick={action.onClick}
          className="inline-flex items-center px-4 py-2 text-sm font-medium text-accent border border-accent rounded-lg hover:bg-accent/10 transition"
        >
          {action.label}
        </button>
      )}
    </div>
  );
};
