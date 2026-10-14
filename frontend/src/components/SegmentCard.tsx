import { applyClip, applyRange, applyVoice, type ClipOption, type Segment, type VoiceOption } from './segment';
import { formatDuration } from './ReviewStage';

type Props = {
  segment: Segment;
  clips: ClipOption[];
  voices: VoiceOption[];
  error?: string;
  active?: boolean;
  onActive?: () => void;
  onChange: (segment: Segment) => void;
};

function durationOf(segment: Segment, voices: VoiceOption[]): number {
  const voice = voices.find((item) => item.id === segment.voiceAssetId);
  const spoken = voice?.durationMS ?? 0;
  const picture = segment.shots.reduce((max, shot) => Math.max(max, shot.sourceOutMS - shot.sourceInMS), 0);
  const caption = segment.captions.reduce((max, item) => Math.max(max, item.endMS - item.startMS), 0);
  return Math.max(spoken, picture, caption);
}

export function SegmentCard({ segment, clips, voices, error, active, onActive, onChange }: Props) {
  const caption = segment.captions[0];
  const lead = segment.shots[0];
  const clip = clips.find((item) => item.id === lead?.clipId);
  return (
    <fieldset className="strip" data-active={active ? 'true' : 'false'} onFocus={() => onActive?.()}>
      <div className="still" aria-hidden="true">
        <span>{clip?.label || '未选镜头'}</span>
        <b>{formatDuration(durationOf(segment, voices))}</b>
      </div>
      <div className="strip-body">
        <p className="purpose">分段 {segment.id}</p>
        <label>
          口播
          <textarea value={segment.voiceText} onChange={(event) => onChange({ ...segment, voiceText: event.target.value })} />
        </label>
        <label>
          配音
          <select value={segment.voiceAssetId} onChange={(event) => onChange(applyVoice(segment, event.target.value, voices))}>
            <option value="">选择预录配音</option>
            {voices.map((voice) => (
              <option key={voice.id} value={voice.id} disabled={!voice.selectable}>
                {voice.selectable ? voice.label : `${voice.label}（${voice.reason ?? '不可用'}）`}
              </option>
            ))}
          </select>
        </label>
        {segment.shots.map((shot, index) => (
          <div className="times" key={index}>
            <label>
              {`镜头 ${index + 1}`}
              <select value={shot.clipId} onChange={(event) => onChange(applyClip(segment, index, event.target.value, clips))}>
                <option value="">选择镜头</option>
                {clips.map((item) => (
                  <option key={item.id} value={item.id} disabled={!item.selectable}>
                    {item.selectable ? item.label : `${item.label}（${item.reason ?? '不可用'}）`}
                  </option>
                ))}
              </select>
            </label>
            <label>
              {`镜头 ${index + 1} 入点（毫秒）`}
              <input type="number" value={shot.sourceInMS} onChange={(event) => onChange(applyRange(segment, index, 'in', event.target.valueAsNumber))} />
            </label>
            <label>
              {`镜头 ${index + 1} 出点（毫秒）`}
              <input type="number" value={shot.sourceOutMS} onChange={(event) => onChange(applyRange(segment, index, 'out', event.target.valueAsNumber))} />
            </label>
          </div>
        ))}
        <button type="button" onClick={() => onChange({ ...segment, shots: [...segment.shots, { clipId: '', sourceInMS: 0, sourceOutMS: 0 }] })}>添加镜头</button>
        <label>
          字幕
          <input value={caption?.text ?? ''} onChange={(event) => onChange({
            ...segment,
            captions: [{ text: event.target.value, startMS: caption?.startMS ?? 0, endMS: caption?.endMS ?? 0 }],
          })} />
        </label>
        <label>
          字幕开始
          <input type="number" value={caption?.startMS ?? 0} onChange={(event) => onChange({
            ...segment,
            captions: [{ text: caption?.text ?? '', startMS: event.target.valueAsNumber, endMS: caption?.endMS ?? 0 }],
          })} />
        </label>
        <label>
          字幕结束
          <input type="number" value={caption?.endMS ?? 0} onChange={(event) => onChange({
            ...segment,
            captions: [{ text: caption?.text ?? '', startMS: caption?.startMS ?? 0, endMS: event.target.valueAsNumber }],
          })} />
        </label>
        {error && <p role="alert">{error}</p>}
      </div>
    </fieldset>
  );
}
