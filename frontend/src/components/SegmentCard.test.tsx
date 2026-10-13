import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { afterEach, expect, test } from 'vitest';

afterEach(() => cleanup());
import { SegmentCard } from './SegmentCard';
import { applyClip, toDraftSegment, type ClipOption, type Segment, type VoiceOption } from './segment';

const clips: ClipOption[] = [
  { id: 'clip-red', label: '红镜头 0–4000', selectable: true, inMS: 0, outMS: 4000 },
  { id: 'clip-ref', label: '参考镜头', selectable: false, reason: '参考素材不能放进成片', inMS: 0, outMS: 4000 },
];
const voices: VoiceOption[] = [
  { id: 'voice-demo', label: '预录口播 3500ms', durationMS: 3500, selectable: true },
];
const initial: Segment = {
  id: 'seg_1',
  voiceText: '旧口播',
  voiceAssetId: 'voice-demo',
  shots: [{ clipId: 'clip-red', sourceInMS: 0, sourceOutMS: 2000 }],
  captions: [{ text: '旧字幕', startMS: 0, endMS: 2000 }],
};

test('更换镜头会传出时间线使用的镜头编号', async () => {
  const user = userEvent.setup();
  let next = initial;
  render(<SegmentCard segment={initial} clips={[
    ...clips,
    { id: 'clip-blue', label: '蓝镜头 0–4000', selectable: true, inMS: 0, outMS: 4000 },
  ]} voices={voices} onChange={(segment) => { next = segment; }} />);
  await user.selectOptions(screen.getByLabelText('镜头 1'), 'clip-blue');
  expect(next.shots[0].clipId).toBe('clip-blue');
  expect(next.shots[0].sourceOutMS).toBe(4000);
  expect(toDraftSegment(next).shots[0]).toEqual({ clip_id: 'clip-blue', source_in_ms: 0, source_out_ms: 4000 });
});

test('参考镜头不能被选中', () => {
  expect(applyClip(initial, 0, 'clip-ref', clips)).toBe(initial);
  render(<SegmentCard segment={initial} clips={clips} voices={voices} onChange={() => {}} />);
  const option = screen.getByRole('option', { name: '参考镜头（参考素材不能放进成片）' }) as HTMLOptionElement;
  expect(option.disabled).toBe(true);
});

test('键盘可以修改口播和区间', async () => {
  const user = userEvent.setup();
  const start: Segment = { ...initial, shots: [{ clipId: 'clip-red', sourceInMS: 0, sourceOutMS: 0 }] };
  let next = start;
  function Harness() {
    const [segment, setSegment] = useState(start);
    return <SegmentCard segment={segment} clips={clips} voices={voices} onChange={(value) => { next = value; setSegment(value); }} />;
  }
  render(<Harness />);
  const voice = screen.getByLabelText('口播');
  await user.type(voice, '新增');
  expect(next.voiceText).toBe('旧口播新增');
  const end = screen.getByLabelText('镜头 1 出点（毫秒）');
  await user.click(end);
  await user.keyboard('3500');
  expect(toDraftSegment(next).shots[0].source_out_ms).toBe(3500);
  expect(toDraftSegment(next).voice_text).toBe('旧口播新增');
  voice.focus();
  expect(document.activeElement).toBe(voice);
});

test('错误用提示显示', () => {
  render(<SegmentCard segment={initial} clips={clips} voices={voices} error="配音超过了这一段的结尾" onChange={() => {}} />);
  expect(screen.getByRole('alert').textContent).toBe('配音超过了这一段的结尾');
});
