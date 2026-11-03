import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { ConfirmButton } from './Confirm';

const setup = (props: { keepOpen?: boolean } = {}) => {
  const onConfirm = vi.fn();
  render(<ConfirmButton label="Delete" confirmLabel="Yes, delete" onConfirm={onConfirm} {...props} />);
  return { onConfirm, user: userEvent.setup() };
};

describe('ConfirmButton focus', () => {
  it('focuses Yes when the confirmation opens', async () => {
    const { user } = setup();
    await user.click(screen.getByRole('button', { name: 'Delete' }));
    expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Yes, delete' }));
  });

  it('returns focus to the first button on Cancel', async () => {
    const { user, onConfirm } = setup();
    await user.click(screen.getByRole('button', { name: 'Delete' }));
    await user.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Delete' }));
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it('returns focus to the first button after Yes, which folds back', async () => {
    const { user, onConfirm } = setup();
    await user.click(screen.getByRole('button', { name: 'Delete' }));
    await user.click(screen.getByRole('button', { name: 'Yes, delete' }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
    expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Delete' }));
  });

  it('stays open after Yes with keepOpen, focus on Yes', async () => {
    const { user } = setup({ keepOpen: true });
    await user.click(screen.getByRole('button', { name: 'Delete' }));
    await user.click(screen.getByRole('button', { name: 'Yes, delete' }));
    expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Yes, delete' }));
  });
});
