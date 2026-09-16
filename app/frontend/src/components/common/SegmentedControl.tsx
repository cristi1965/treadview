import React from 'react';

export interface Segment<T extends string> {
  id: T;
  label: string;
}

interface SegmentedControlProps<T extends string> {
  /** NoInfer pins T to `value`, so mapped segment arrays and setState handlers can't widen it. */
  segments: Array<Segment<NoInfer<T>>>;
  value: T;
  onChange: (value: NoInfer<T>) => void;
  size?: 'sm' | 'md';
  className?: string;
  'aria-label'?: string;
}

/** iOS-style segmented control: capsule track, raised selected pill, hairline-free. */
export function SegmentedControl<T extends string>({
  segments,
  value,
  onChange,
  size = 'md',
  className = '',
  'aria-label': ariaLabel,
}: SegmentedControlProps<T>) {
  const pad = size === 'sm' ? 'px-2.5 py-1 text-[11px]' : 'px-3.5 py-1.5 text-[13px]';

  return (
    <div
      role="tablist"
      aria-label={ariaLabel}
      className={`inline-flex gap-0.5 rounded-ios-sm bg-ios-fill p-0.5 ${className}`}
    >
      {segments.map((segment) => {
        const active = segment.id === value;
        return (
          <button
            key={segment.id}
            type="button"
            role="tab"
            aria-selected={active}
            onClick={() => onChange(segment.id)}
            className={`rounded-[6px] font-medium transition ${pad} ${
              active
                ? 'bg-ios-fill-2 text-ios-label shadow-[0_1px_2px_rgba(0,0,0,0.35)]'
                : 'text-ios-label-2 hover:text-ios-label'
            }`}
          >
            {segment.label}
          </button>
        );
      })}
    </div>
  );
}
