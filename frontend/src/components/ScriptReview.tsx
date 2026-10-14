import type { Candidate, ScriptSegment, ShotNote } from '../api';
import { formatDuration } from './ReviewStage';

type Props = {
  candidates: Candidate[];
  notes: ShotNote[];
  activeSegmentId?: string;
  onSelect?: (segment: ScriptSegment, candidate: Candidate) => void;
  onEditCandidate: (candidate: Candidate) => void;
  onRetranslate: (candidate: Candidate) => void;
  onAdopt: (candidate: Candidate) => void;
  onChangeNote: (note: ShotNote) => void;
  onSaveNote: (note: ShotNote) => void;
};

export function ScriptReview({
  candidates,
  notes,
  activeSegmentId,
  onSelect,
  onEditCandidate,
  onRetranslate,
  onAdopt,
  onChangeNote,
  onSaveNote,
}: Props) {
  return (
    <div className="review">
      {candidates.map((candidate) => (
        <article className="candidate" key={candidate.id}>
          <header className="candidate-head">
            <p>钩子：{candidate.hook_type}</p>
            <h3>中文回译</h3>
            <p className="zh">{candidate.back_translation}</p>
            {!candidate.translation_of && <p role="status">回译已失效</p>}
          </header>
          {candidate.segments.map((segment) => (
            <div
              className="strip"
              data-active={segment.id === activeSegmentId ? 'true' : 'false'}
              key={segment.id}
            >
              <div className="still" aria-hidden="true">
                <span>{segment.clip_ids[0] || '未选镜头'}</span>
                <b>{formatDuration(segment.estimated_ms)}</b>
              </div>
              <div className="strip-body">
                <p className="purpose">{segment.purpose}</p>
                <label>
                  {`印尼语口播 ${segment.id}`}
                  <textarea
                    value={segment.voice_text}
                    onFocus={() => onSelect?.(segment, candidate)}
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
            </div>
          ))}
          <div className="actions">
            <button type="button" onClick={() => onRetranslate(candidate)}>重新生成回译</button>
            <button type="button" onClick={() => onAdopt(candidate)}>采用这条候选</button>
          </div>
        </article>
      ))}
      {notes.map((note) => (
        <fieldset className="strip note" key={note.clip_id}>
          <div className="still" aria-hidden="true">
            <span>{note.subject || '待确认'}</span>
          </div>
          <div className="strip-body">
            <p className="purpose">镜头理解 {note.clip_id}</p>
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
          </div>
        </fieldset>
      ))}
    </div>
  );
}
