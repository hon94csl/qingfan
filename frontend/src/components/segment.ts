export type Shot = { clipId: string; sourceInMS: number; sourceOutMS: number };
export type Segment = {
  id: string;
  voiceText: string;
  voiceAssetId: string;
  shots: Shot[];
  captions: { text: string; startMS: number; endMS: number }[];
};
export type ClipOption = {
  id: string;
  label: string;
  selectable: boolean;
  reason?: string;
  inMS: number;
  outMS: number;
};
export type VoiceOption = { id: string; label: string; durationMS: number; selectable: boolean; reason?: string };

export type DraftSegment = {
  id: string;
  role: string;
  voice_text: string;
  voice_asset_id: string;
  shots: { clip_id: string; source_in_ms: number; source_out_ms: number }[];
  captions: { text: string; start_ms: number; end_ms: number }[];
};

export function applyClip(segment: Segment, index: number, clipId: string, clips: ClipOption[]): Segment {
  const clip = clips.find((item) => item.id === clipId);
  if (!clip || !clip.selectable) return segment;
  const shots = segment.shots.map((shot, i) => i === index ? { clipId, sourceInMS: clip.inMS, sourceOutMS: clip.outMS } : shot);
  return { ...segment, shots };
}

export function applyRange(segment: Segment, index: number, edge: 'in' | 'out', value: number): Segment {
  if (!Number.isFinite(value)) return segment;
  const shots = segment.shots.map((shot, i) => {
    if (i !== index) return shot;
    return edge === 'in' ? { ...shot, sourceInMS: value } : { ...shot, sourceOutMS: value };
  });
  return { ...segment, shots };
}

export function applyVoice(segment: Segment, voiceAssetId: string, voices: VoiceOption[]): Segment {
  const voice = voices.find((item) => item.id === voiceAssetId);
  if (!voice || !voice.selectable) return segment;
  return { ...segment, voiceAssetId };
}

export function segmentFromDraft(segment: DraftSegment): Segment {
  return {
    id: segment.id,
    voiceText: segment.voice_text,
    voiceAssetId: segment.voice_asset_id,
    shots: segment.shots.map((shot) => ({
      clipId: shot.clip_id,
      sourceInMS: shot.source_in_ms,
      sourceOutMS: shot.source_out_ms,
    })),
    captions: segment.captions.map((caption) => ({
      text: caption.text,
      startMS: caption.start_ms,
      endMS: caption.end_ms,
    })),
  };
}

export function mergeDraftSegments(segment: Segment, existing: DraftSegment[] | undefined): DraftSegment[] {
  const first = toDraftSegment(segment);
  if (existing?.[0]?.role) first.role = existing[0].role;
  return [first, ...(existing ?? []).slice(1)];
}

export function toDraftSegment(segment: Segment): DraftSegment {
  return {
    id: segment.id,
    role: 'hook',
    voice_text: segment.voiceText,
    voice_asset_id: segment.voiceAssetId,
    shots: segment.shots.map((shot) => ({
      clip_id: shot.clipId,
      source_in_ms: shot.sourceInMS,
      source_out_ms: shot.sourceOutMS,
    })),
    captions: segment.captions.map((caption) => ({
      text: caption.text,
      start_ms: caption.startMS,
      end_ms: caption.endMS,
    })),
  };
}
