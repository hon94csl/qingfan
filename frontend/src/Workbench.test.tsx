import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, test } from 'vitest';
import { Workbench } from './Workbench';

afterEach(() => cleanup());

test('浏览器里不能绑定发布或预览回流', async () => {
  const user = userEvent.setup();
  render(<Workbench />);
  expect(screen.getByRole('region', { name: '竖屏预览' })).toBeTruthy();
  expect(screen.getByText('写出商品事实后，这里会按段显示口播和镜头。')).toBeTruthy();
  expect(screen.queryByRole('textbox', { name: '视频编号' })).toBeNull();
  expect(screen.queryByRole('textbox', { name: '文件路径' })).toBeNull();
  await user.click(screen.getByRole('button', { name: '发布与回流' }));
  await user.type(screen.getByLabelText('视频编号'), '7123');
  expect((screen.getByLabelText('视频编号') as HTMLInputElement).value).toBe('7123');
  await user.click(screen.getByRole('button', { name: '预览回流' }));
  expect(screen.getByRole('alert').textContent).toBe('预览回流需要在桌面应用里打开');
  await user.click(screen.getByRole('button', { name: '绑定发布' }));
  expect(screen.getByRole('alert').textContent).toBe('绑定发布需要在桌面应用里打开');
  await user.click(screen.getByRole('button', { name: '导入回流' }));
  expect(screen.getByRole('alert').textContent).toBe('导入回流需要在桌面应用里打开');
});
