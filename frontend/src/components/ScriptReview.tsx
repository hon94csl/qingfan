import type { Candidate, ShotNote } from '../api';

type Props = {
  candidates: Candidate[];
  notes: ShotNote[];
  onEditCandidate: (candidate: Candidate) => void;
  onRetranslate: (candidate: Candidate) => void;
  onAdopt: (candidate: Candidate) => void;
  onChangeNote: (note: ShotNote) => void;
  onSaveNote: (note: ShotNote) => void;
};

export function ScriptReview({ candidates, notes, onEditCandidate, onRetranslate, onAdopt, onChangeNote, onSaveNote }: Props) {
  return (
    <div>
      {candidates.map((candidate) => (
        <fieldset className="card" key={candidate.id}>
          <legend>候选 {candidate.id}</legend>
          <p>钩子：{candidate.hook_type}</p>
          {candidate.segments.map((segment) => (
            <div key={segment.id}>
              <label>
                {`印尼语口播 ${segment.id}`}
                <textarea
                  value={segment.voice_text}
                  onChange={(event) => onEditCandidate({
                    ...candidate,
                    translation_of: '',
                    segments: candidate.segments.map((item) => item.id === segment.id ? { ...item, voice_text: event.target.value } : item),
                  })}
                />
              </label>
              <p>引用事实：{segment.fact_ids.join('、') || '无'}</p>
              <p>引用镜头：{segment.clip_ids.join('、') || '无'}</p>
            </div>
          ))}
          <p>中文回译：{candidate.back_translation}</p>
          {!candidate.translation_of && <p role="status">回译已失效</p>}
          <div className="grid">
            <button type="button" onClick={() => onRetranslate(candidate)}>重新生成回译</button>
            <button type="button" onClick={() => onAdopt(candidate)}>采用这条候选</button>
          </div>
        </fieldset>
      ))}
      {notes.map((note) => (
        <fieldset className="card" key={note.clip_id}>
          <legend>镜头理解 {note.clip_id}</legend>
          <label>
            主体
            <input value={note.subject} onChange={(event) => onChangeNote({ ...note, subject: event.target.value })} />
          </label>
          <label>
            动作
            <input value={note.action} onChange={(event) => onChangeNote({ ...note, action: event.target.value })} />
          </label>
          <p>用途：{note.purpose || '待确认'}</p>
          <p>卖点：{(note.selling_points ?? []).join('、') || '待确认'}</p>
          <button type="button" onClick={() => onSaveNote(note)}>保存修正</button>
        </fieldset>
      ))}
    </div>
  );
}
