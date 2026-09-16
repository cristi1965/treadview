import React, { useState } from 'react';
import { Avatar } from './Avatar';

interface PersonAvatarProps {
  name: string;
  slug?: string;
  size?: number;
  className?: string;
  role?: 'CEO' | 'FED' | 'GURU' | 'ANALYST';
}

export const PersonAvatar: React.FC<PersonAvatarProps> = ({
  name,
  slug,
  size = 40,
  className = '',
  role = 'GURU',
}) => {
  return (
    <Avatar
      name={name}
      slug={slug}
      size={size}
      className={className}
    />
  );
};
