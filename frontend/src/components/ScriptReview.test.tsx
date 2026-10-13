import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { afterEach, expect, test } from 'vitest';
import type { Candidate, ShotNote } from '../api';
import { ScriptReview } from './ScriptReview';
import { mergeDraftSegments, type DraftSegment, type Segment } from './segment';

afterEach(() => cleanup());

const candidate: Candidate = {
  id: 'c1',
  hook_type: 'pain_question',
  back_translation: '这是固定回译，需要人工核对。',
  translation_of: 'abc',
  segments: [{
    id: 'seg_1',
    role: 'hook',
    purpose: '提出问题',
    fact_ids: ['fact-1'],
    voice_text: 'Lihat barang ini.',
    clip_ids: ['clip-red'],
    estimated_ms: 1000,
  }],
};
const note: ShotNote = {
  clip_id: 'clip-red',
  subject: '待确认主体',
  action: '待确认动作',
  purpose: '展示商品',
  selling_points: [],
  source_in_ms: 0,
  source_out_ms: 4000,
  manual: false,
};

test('修改口播会使回译失效，采用仍交出当前文案', async () => {
  const user = userEvent.setup();
  let adopted: Candidate | null = null;
  function Harness() {
    const [current, setCurrent] = useState(candidate);
    return <ScriptReview
      candidates={[current]}
      notes={[]}
      onEditCandidate={setCurrent}
      onRetranslate={() => {}}
      onAdopt={(value) => { adopted = value; }}
      onChangeNote={() => {}}
      onSaveNote={() => {}}
    />;
  }
  render(<Harness />);
  expect(screen.queryByText('回译已失效')).toBeNull();
  const voice = screen.getByLabelText('印尼语口播 seg_1');
  await user.type(voice, 'x');
  expect(screen.getByText('回译已失效').textContent).toBe('回译已失效');
  await user.click(screen.getByRole('button', { name: '采用这条候选' }));
  expect(adopted?.translation_of).toBe('');
  expect(adopted?.segments[0].voice_text).toBe('Lihat barang ini.x');
  expect(screen.getByText('引用事实：fact-1').textContent).toContain('fact-1');
  expect(screen.getByText('引用镜头：clip-red').textContent).toContain('clip-red');
});

test('保存修正交出改过的主体', async () => {
  const user = userEvent.setup();
  let saved: ShotNote | null = null;
  function Harness() {
    const [current, setCurrent] = useState(note);
    return <ScriptReview
      candidates={[]}
      notes={[current]}
      onEditCandidate={() => {}}
      onRetranslate={() => {}}
      onAdopt={() => {}}
      onChangeNote={setCurrent}
      onSaveNote={(value) => { saved = value; }}
    />;
  }
  render(<Harness />);
  const subject = screen.getByLabelText('主体');
  await user.type(subject, '瓶');
  await user.click(screen.getByRole('button', { name: '保存修正' }));
  expect(saved?.subject).toBe('待确认主体瓶');
  expect(saved?.clip_id).toBe('clip-red');
});

test('保存草稿保留后面的分段和配方', () => {
  const segment: Segment = {
    id: 'seg_1',
    voiceText: '改过的口播',
    voiceAssetId: 'voice-1',
    shots: [{ clipId: 'clip-red', sourceInMS: 0, sourceOutMS: 800 }],
    captions: [{ text: '改过的口播', startMS: 0, endMS: 800 }],
  };
  const existing: DraftSegment[] = [
    {
      id: 'seg_1',
      role: 'hook',
      voice_text: '旧口播',
      voice_asset_id: 'voice-1',
      shots: [{ clip_id: 'clip-red', source_in_ms: 0, source_out_ms: 800 }],
      captions: [{ text: '旧口播', start_ms: 0, end_ms: 800 }],
    },
    {
      id: 'seg_2',
      role: 'demo',
      voice_text: '第二段',
      voice_asset_id: 'voice-2',
      shots: [{ clip_id: 'clip-blue', source_in_ms: 0, source_out_ms: 900 }],
      captions: [{ text: '第二段', start_ms: 0, end_ms: 900 }],
    },
  ];
  const merged = mergeDraftSegments(segment, existing);
  expect(merged.length).toBe(2);
  expect(merged[0].role).toBe('hook');
  expect(merged[0].voice_text).toBe('改过的口播');
  expect(merged[1].id).toBe('seg_2');
  expect(merged[1].voice_text).toBe('第二段');
});
