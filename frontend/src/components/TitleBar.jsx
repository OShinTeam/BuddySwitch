import { WindowClose, WindowMinimise, WindowToggleMaximise } from '../../wailsjs/go/service/App'
import { Icons } from './ui'

/** 无边框窗口的自定义标题栏。 */
export default function TitleBar({ title, subtitle }) {
  return (
    <div
      className="wails-drag flex h-9 shrink-0 items-center justify-between border-b border-slate-200 bg-white px-3"
      onDoubleClick={WindowToggleMaximise}
    >
      <div className="flex min-w-0 items-baseline gap-2">
        <span className="text-[13px] font-semibold text-slate-800">{title}</span>
        {subtitle ? <span className="truncate text-[11px] text-slate-400">{subtitle}</span> : null}
      </div>

      <div className="wails-no-drag flex items-center gap-0.5">
        <button
          type="button"
          onClick={WindowMinimise}
          className="flex h-7 w-8 items-center justify-center rounded text-slate-500 hover:bg-slate-100"
          title="minimise"
        >
          <Icons.minimise className="h-3.5 w-3.5" />
        </button>
        <button
          type="button"
          onClick={WindowToggleMaximise}
          className="flex h-7 w-8 items-center justify-center rounded text-slate-500 hover:bg-slate-100"
          title="maximise"
        >
          <Icons.maximise className="h-3.5 w-3.5" />
        </button>
        <button
          type="button"
          onClick={WindowClose}
          className="flex h-7 w-8 items-center justify-center rounded text-slate-500 hover:bg-rose-500 hover:text-white"
          title="close"
        >
          <Icons.close className="h-3.5 w-3.5" />
        </button>
      </div>
    </div>
  )
}
