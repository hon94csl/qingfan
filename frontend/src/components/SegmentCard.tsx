import { applyClip, applyRange, applyVoice, type ClipOption, type Segment, type VoiceOption } from './segment';

type Props = {
  segment: Segment;
  clips: ClipOption[];
  voices: VoiceOption[];
  error?: string;
  onChange: (segment: Segment) => void;
};

export function SegmentCard({ segment, clips, voices, error, onChange }: Props) {
  const caption = segment.captions[0];
  return (
    <fieldset className="card">
      <legend>分段 {segment.id}</legend>
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
        <div className="grid" key={index}>
          <label>
            {`镜头 ${index + 1}`}
            <select value={shot.clipId} onChange={(event) => onChange(applyClip(segment, index, event.target.value, clips))}>
              <option value="">选择镜头</option>
              {clips.map((clip) => (
                <option key={clip.id} value={clip.id} disabled={!clip.selectable}>
                  {clip.selectable ? clip.label : `${clip.label}（${clip.reason ?? '不可用'}）`}
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
    </fieldset>
  );
}
