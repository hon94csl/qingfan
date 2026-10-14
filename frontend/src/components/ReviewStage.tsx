export type StageView = {
  line: string;
  translation: string;
  shot: string;
  durationMS: number;
};

export function formatDuration(ms: number): string {
  if (!Number.isFinite(ms) || ms <= 0) return '时长未定';
  return `${(ms / 1000).toFixed(1)} 秒`;
}

export function ReviewStage({ view }: { view: StageView | null }) {
  return (
    <section className="phone" aria-label="竖屏预览">
      <div className="screen">
        {view ? (
          <>
            <p className="burn">{formatDuration(view.durationMS)}</p>
            <p className="shot">{view.shot || '镜头还没选定'}</p>
            <div className="lower">
              {view.translation && <p className="zh">{view.translation}</p>}
              <p className="line">{view.line || '这一段还没有口播'}</p>
            </div>
          </>
        ) : (
          <p className="invite">写出商品事实后，这里会按段显示口播和镜头。</p>
        )}
      </div>
    </section>
  );
}
