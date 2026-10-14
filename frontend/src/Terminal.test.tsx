import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, test } from 'vitest';
import { formatDuration, Terminal, videoLinks } from './Terminal';

afterEach(() => cleanup());

test('出片先填商品再选素材，然后才开始自动出片', async () => {
  const user = userEvent.setup();
  render(<Terminal initialEmail="ops@example.com" />);
  expect(screen.getByRole('heading', { name: '素材库' })).toBeTruthy();
  expect(screen.getByRole('region', { name: '云端罗盘' })).toBeTruthy();
  expect(screen.getByText('还没有平台素材')).toBeTruthy();
  await user.click(screen.getByRole('tab', { name: '本地素材' }));
  expect(screen.getByRole('region', { name: '本地素材' })).toBeTruthy();
  await user.click(screen.getByRole('button', { name: '放入素材库' }));
  expect(screen.getByRole('alert').textContent).toBe('先填写商品');
  await user.type(screen.getByLabelText('商品'), '面霜');
  await user.type(screen.getByLabelText('文件路径'), '/tmp/a.mp4');
  await user.click(screen.getByRole('button', { name: '选择文件' }));
  expect(screen.getByRole('alert').textContent).toBe('选择文件需要在桌面应用里打开');
  await user.click(screen.getByRole('button', { name: '放入素材库' }));
  expect(screen.getByRole('cell', { name: 'a.mp4' })).toBeTruthy();
  expect(screen.getByRole('cell', { name: '—' })).toBeTruthy();
  expect(screen.getByText('已列入素材库。文件导入需要在桌面应用里打开。')).toBeTruthy();

  await user.click(screen.getByRole('button', { name: '出片' }));
  expect(screen.queryByRole('alert')).toBeNull();
  expect(screen.getByRole('region', { name: '自动出片' })).toBeTruthy();
  expect(screen.getByText(/填写商品、视频链接和提示词/)).toBeTruthy();
  expect(screen.getByLabelText('视频链接')).toBeTruthy();
  expect(screen.getByLabelText('提示词')).toBeTruthy();
  expect(screen.queryByLabelText('已确认的商品事实')).toBeNull();
  expect(screen.getByRole('checkbox', { name: 'a.mp4' })).toBeTruthy();
  await user.clear(screen.getByLabelText('商品'));
  await user.click(screen.getByRole('button', { name: '开始自动混剪' }));
  expect(screen.getByRole('alert').textContent).toBe('先填写商品');
  await user.type(screen.getByLabelText('商品'), '面霜');
  await user.click(screen.getByRole('button', { name: '开始自动混剪' }));
  expect(screen.getByRole('alert').textContent).toBe('先选择素材');
  await user.click(screen.getByRole('checkbox', { name: 'a.mp4' }));
  await user.click(screen.getByRole('button', { name: '开始自动混剪' }));
  expect(screen.getByRole('alert').textContent).toBe('自动出片需要在桌面应用里打开');
});

test('视频链接和提示词走自动混剪', async () => {
  const user = userEvent.setup();
  render(<Terminal initialEmail="ops@example.com" />);
  await user.click(screen.getByRole('button', { name: '出片' }));
  await user.type(screen.getByLabelText('商品'), '面霜');
  await user.type(screen.getByLabelText('视频链接'), 'file:///tmp/a.mp4');
  await user.click(screen.getByRole('button', { name: '开始自动混剪' }));
  expect(screen.getByRole('alert').textContent).toBe('先填写 http 或 https 视频链接');
  await user.clear(screen.getByLabelText('视频链接'));
  await user.type(screen.getByLabelText('视频链接'), 'https://cdn.example.com/a.mp4');
  await user.click(screen.getByRole('button', { name: '开始自动混剪' }));
  expect(screen.getByRole('alert').textContent).toBe('先填写提示词');
  await user.type(screen.getByLabelText('提示词'), '先讲痛点，再演示开盖');
  await user.click(screen.getByRole('button', { name: '开始自动混剪' }));
  expect(screen.getByRole('alert').textContent).toBe('自动混剪需要在桌面应用里打开');
  expect(videoLinks(' https://cdn.example.com/a.mp4 \n https://cdn.example.com/b.mp4 ')).toEqual([
    'https://cdn.example.com/a.mp4',
    'https://cdn.example.com/b.mp4',
  ]);
});

test('时长只显示探测到的毫秒', () => {
  expect(formatDuration(undefined)).toBe('—');
  expect(formatDuration(0)).toBe('—');
  expect(formatDuration(4000)).toBe('4 秒');
  expect(formatDuration(3500)).toBe('3.5 秒');
});

test('平台罗盘由当前账号创建自己的店铺', async () => {
  const user = userEvent.setup();
  render(<Terminal initialEmail="ops@example.com" />);
  expect(screen.getByText(/一个账号可以创建多家/)).toBeTruthy();
  expect(screen.getByText('还没有店铺')).toBeTruthy();
  expect(screen.getByText('还没有平台素材')).toBeTruthy();
  expect(screen.queryByRole('button', { name: '添加店铺' })).toBeNull();
  expect(screen.queryByLabelText('授权码')).toBeNull();
  await user.click(screen.getByRole('button', { name: '创建店铺' }));
  expect(screen.getByRole('alert').textContent).toBe('先填写店铺');
  await user.type(screen.getByLabelText('店铺'), '一店');
  await user.click(screen.getByRole('button', { name: '创建店铺' }));
  expect(screen.getByRole('alert').textContent).toBe('创建店铺需要在桌面应用里打开');
});

test('出片页可以手填发布并预览回流', async () => {
  const user = userEvent.setup();
  render(<Terminal initialEmail="ops@example.com" />);
  await user.click(screen.getByRole('button', { name: '出片' }));
  expect(screen.getByRole('region', { name: '发布与回流' })).toBeTruthy();
  expect(screen.getByText('还没有调用记录。没有换成货币，也没有按采用成片分摊。')).toBeTruthy();
  await user.click(screen.getByRole('button', { name: '绑定发布' }));
  expect(screen.getByRole('alert').textContent).toBe('绑定发布需要在桌面应用里打开');
  await user.click(screen.getByRole('button', { name: '预览回流' }));
  expect(screen.getByRole('alert').textContent).toBe('预览回流需要在桌面应用里打开');
  await user.click(screen.getByRole('button', { name: '导入回流' }));
  expect(screen.getByRole('alert').textContent).toBe('导入回流需要在桌面应用里打开');
});

test('没有登录时不能进入素材库', async () => {
  const user = userEvent.setup();
  render(<Terminal />);
  expect(screen.queryByRole('heading', { name: '素材库' })).toBeNull();
  expect(screen.getByRole('heading', { name: '登录' })).toBeTruthy();
  expect(screen.getByText('账号由管理后台开通。登录后才能使用。')).toBeTruthy();
  expect(screen.queryByRole('button', { name: '注册账号' })).toBeNull();
  await user.click(screen.getByRole('button', { name: '登录' }));
  expect(screen.getByRole('alert').textContent).toBe('请在桌面窗口里登录');
});

test('开放注册时可以切到注册', async () => {
  const user = userEvent.setup();
  render(<Terminal registrationOpen />);
  expect(screen.queryByRole('heading', { name: '素材库' })).toBeNull();
  await user.click(screen.getByRole('button', { name: '注册账号' }));
  expect(screen.getByRole('heading', { name: '注册' })).toBeTruthy();
  expect(screen.getByText('注册后即可使用。还没有收款。')).toBeTruthy();
  await user.type(screen.getByLabelText('密码'), 'long-test-password');
  await user.type(screen.getByLabelText('确认密码'), 'other-long-password');
  await user.click(screen.getByRole('button', { name: '注册' }));
  expect(screen.getByRole('alert').textContent).toBe('两次输入的密码不一致');
  await user.clear(screen.getByLabelText('确认密码'));
  await user.type(screen.getByLabelText('确认密码'), 'long-test-password');
  await user.click(screen.getByRole('button', { name: '注册' }));
  expect(screen.getByRole('alert').textContent).toBe('请在桌面窗口里登录');
});
