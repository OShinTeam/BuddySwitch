// 一套最小的界面原语：按钮、开关、抽屉、表单控件、图标。
// 刻意不引入组件库——这个应用的界面元素很少，手写反而更可控。

/**
 * 浮层层级。
 *
 * 面板之间会互相叠：上游面板里点「应用」要在它之上再开一个对话框，点「删除」
 * 还要在上面再叠一层确认框。全都写死同一个 z-index 的话，谁在 DOM 里靠后就谁赢，
 * 结果就是「点了应用得先关掉上游面板才看得见」。所以这里显式分层。
 */
export const LAYERS = {
  // 面板本身（设置、备份、上游目录、模型编辑）
  base: 40,
  // 从面板里再打开的对话框（应用、拉取、手动添加、新增/编辑上游）
  raised: 50,
  // 确认框永远在最上面
  top: 60,
}

function Svg({ children, className = 'h-4 w-4' }) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      className={className}
      aria-hidden="true"
    >
      {children}
    </svg>
  )
}

export const Icons = {
  refresh: (p) => (
    <Svg {...p}>
      <path d="M21 12a9 9 0 1 1-2.64-6.36" />
      <path d="M21 3v6h-6" />
    </Svg>
  ),
  plus: (p) => (
    <Svg {...p}>
      <path d="M12 5v14M5 12h14" />
    </Svg>
  ),
  trash: (p) => (
    <Svg {...p}>
      <path d="M3 6h18" />
      <path d="M8 6V4h8v2" />
      <path d="M19 6l-1 14H6L5 6" />
      <path d="M10 11v5M14 11v5" />
    </Svg>
  ),
  pencil: (p) => (
    <Svg {...p}>
      <path d="M12 20h9" />
      <path d="M16.5 3.5a2.12 2.12 0 0 1 3 3L7 19l-4 1 1-4Z" />
    </Svg>
  ),
  sliders: (p) => (
    <Svg {...p}>
      <path d="M4 21v-7M4 10V3M12 21v-9M12 8V3M20 21v-5M20 12V3" />
      <path d="M1 14h6M9 8h6M17 16h6" />
    </Svg>
  ),
  folder: (p) => (
    <Svg {...p}>
      <path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z" />
    </Svg>
  ),
  download: (p) => (
    <Svg {...p}>
      <path d="M12 3v12" />
      <path d="M7 11l5 5 5-5" />
      <path d="M5 21h14" />
    </Svg>
  ),
  close: (p) => (
    <Svg {...p}>
      <path d="M18 6 6 18M6 6l12 12" />
    </Svg>
  ),
  minimise: (p) => (
    <Svg {...p}>
      <path d="M5 12h14" />
    </Svg>
  ),
  maximise: (p) => (
    <Svg {...p}>
      <rect x="5" y="5" width="14" height="14" rx="2" />
    </Svg>
  ),
  search: (p) => (
    <Svg {...p}>
      <circle cx="11" cy="11" r="7" />
      <path d="m20 20-3.5-3.5" />
    </Svg>
  ),
  history: (p) => (
    <Svg {...p}>
      <path d="M3 12a9 9 0 1 0 3-6.7" />
      <path d="M3 4v5h5" />
      <path d="M12 8v4l3 2" />
    </Svg>
  ),
  beaker: (p) => (
    <Svg {...p}>
      <path d="M9 3h6" />
      <path d="M10 3v6l-5 10a2 2 0 0 0 1.8 3h10.4A2 2 0 0 0 19 19l-5-10V3" />
    </Svg>
  ),
  check: (p) => (
    <Svg {...p}>
      <path d="M20 6 9 17l-5-5" />
    </Svg>
  ),
  alert: (p) => (
    <Svg {...p}>
      <path d="M12 9v4M12 17h.01" />
      <path d="M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0Z" />
    </Svg>
  ),
  reveal: (p) => (
    <Svg {...p}>
      <path d="M15 3h6v6" />
      <path d="M10 14 21 3" />
      <path d="M21 14v5a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5" />
    </Svg>
  ),
  link: (p) => (
    <Svg {...p}>
      <path d="M9 15l6-6" />
      <path d="M11 6l1-1a4 4 0 0 1 6 6l-1 1" />
      <path d="M13 18l-1 1a4 4 0 0 1-6-6l1-1" />
    </Svg>
  ),
  layers: (p) => (
    <Svg {...p}>
      <path d="m12 2 9 5-9 5-9-5Z" />
      <path d="m3 12 9 5 9-5" />
      <path d="m3 17 9 5 9-5" />
    </Svg>
  ),
  chevron: (p) => (
    <Svg {...p}>
      <path d="m9 6 6 6-6 6" />
    </Svg>
  ),
  server: (p) => (
    <Svg {...p}>
      <rect x="3" y="4" width="18" height="7" rx="2" />
      <rect x="3" y="13" width="18" height="7" rx="2" />
      <path d="M7 7.5h.01M7 16.5h.01" />
    </Svg>
  ),
  forward: (p) => (
    <Svg {...p}>
      <path d="M5 12h13" />
      <path d="m13 6 6 6-6 6" />
    </Svg>
  ),
}

const buttonVariants = {
  primary:
    'bg-indigo-600 text-white hover:bg-indigo-500 active:bg-indigo-700 shadow-sm disabled:bg-indigo-300',
  default:
    'bg-white text-slate-700 ring-1 ring-slate-200 hover:bg-slate-50 active:bg-slate-100 disabled:text-slate-400',
  ghost: 'text-slate-600 hover:bg-slate-100 active:bg-slate-200 disabled:text-slate-300',
  danger: 'bg-rose-600 text-white hover:bg-rose-500 active:bg-rose-700 disabled:bg-rose-300',
}

export function Button({
  variant = 'default',
  icon: Icon,
  children,
  className = '',
  size = 'md',
  ...rest
}) {
  const sizing = size === 'sm' ? 'h-7 px-2.5 text-xs gap-1.5' : 'h-8 px-3 text-[13px] gap-2'
  return (
    <button
      type="button"
      className={`inline-flex shrink-0 items-center justify-center rounded-md font-medium transition-colors disabled:cursor-not-allowed ${sizing} ${buttonVariants[variant]} ${className}`}
      {...rest}
    >
      {Icon ? <Icon className={size === 'sm' ? 'h-3.5 w-3.5' : 'h-4 w-4'} /> : null}
      {children}
    </button>
  )
}

export function IconButton({ title, active = false, className = '', children, ...rest }) {
  return (
    <button
      type="button"
      title={title}
      className={`inline-flex h-7 w-7 items-center justify-center rounded-md transition-colors ${
        active ? 'bg-indigo-50 text-indigo-600' : 'text-slate-500 hover:bg-slate-100 hover:text-slate-700'
      } ${className}`}
      {...rest}
    >
      {children}
    </button>
  )
}

export function Switch({ checked, onChange, disabled = false, title }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      title={title}
      disabled={disabled}
      onClick={() => !disabled && onChange(!checked)}
      className={`relative inline-flex h-[18px] w-[32px] shrink-0 items-center rounded-full transition-colors ${
        checked ? 'bg-indigo-600' : 'bg-slate-300'
      } ${disabled ? 'cursor-not-allowed opacity-50' : 'cursor-pointer'}`}
    >
      <span
        className={`inline-block h-[14px] w-[14px] transform rounded-full bg-white shadow transition-transform ${
          checked ? 'translate-x-[16px]' : 'translate-x-[2px]'
        }`}
      />
    </button>
  )
}

export function Chip({ children, className = '' }) {
  return (
    <span
      className={`inline-flex items-center gap-1 rounded-full px-1.5 py-0.5 text-[11px] font-medium ring-1 ring-inset ${className}`}
    >
      {children}
    </span>
  )
}

export function Spinner({ className = 'h-3.5 w-3.5' }) {
  return (
    <svg className={`animate-spin ${className}`} viewBox="0 0 24 24" fill="none">
      <circle cx="12" cy="12" r="9" stroke="currentColor" strokeOpacity="0.25" strokeWidth="3" />
      <path d="M21 12a9 9 0 0 0-9-9" stroke="currentColor" strokeWidth="3" strokeLinecap="round" />
    </svg>
  )
}

export function Drawer({
  open,
  onClose,
  title,
  subtitle,
  width = 'w-[560px]',
  layer = LAYERS.base,
  footer,
  children,
}) {
  if (!open) return null
  return (
    <div className="fixed inset-0 flex justify-end" style={{ zIndex: layer }}>
      <div className="absolute inset-0 bg-slate-900/20 backdrop-blur-[1px]" onClick={onClose} />
      <div
        className={`relative flex h-full ${width} max-w-[92vw] flex-col border-l border-slate-200 bg-white shadow-2xl`}
      >
        <header className="flex items-start justify-between gap-4 border-b border-slate-200 px-5 py-3.5">
          <div className="min-w-0">
            <h2 className="truncate text-sm font-semibold text-slate-800">{title}</h2>
            {subtitle ? <p className="mt-1 text-xs leading-relaxed text-slate-500">{subtitle}</p> : null}
          </div>
          <IconButton title="close" onClick={onClose}>
            <Icons.close className="h-4 w-4" />
          </IconButton>
        </header>
        <div className="flex-1 overflow-y-auto px-5 py-4">{children}</div>
        {footer ? (
          <footer className="flex items-center justify-end gap-2 border-t border-slate-200 bg-slate-50 px-5 py-3">
            {footer}
          </footer>
        ) : null}
      </div>
    </div>
  )
}

export function Field({ label, hint, children }) {
  return (
    <label className="block">
      <span className="mb-1.5 block text-xs font-medium text-slate-600">{label}</span>
      {children}
      {hint ? <span className="mt-1 block text-[11px] leading-relaxed text-slate-400">{hint}</span> : null}
    </label>
  )
}

const inputClass =
  'w-full rounded-md border-0 bg-white px-2.5 py-1.5 text-[13px] text-slate-800 ring-1 ring-inset ring-slate-200 outline-none transition placeholder:text-slate-400 focus:ring-2 focus:ring-indigo-500 disabled:bg-slate-50 disabled:text-slate-400'

export function TextInput({ className = '', ...rest }) {
  return <input className={`${inputClass} ${className}`} {...rest} />
}

export function TextArea({ className = '', ...rest }) {
  return <textarea className={`${inputClass} resize-none ${className}`} {...rest} />
}

export function Select({ className = '', children, ...rest }) {
  return (
    <select className={`${inputClass} cursor-pointer pr-8 ${className}`} {...rest}>
      {children}
    </select>
  )
}

export function EmptyState({ icon: Icon = Icons.layers, title, desc, action }) {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-3 px-8 text-center">
      <div className="flex h-11 w-11 items-center justify-center rounded-full bg-slate-100 text-slate-400">
        <Icon className="h-5 w-5" />
      </div>
      <div>
        <p className="text-sm font-medium text-slate-700">{title}</p>
        {desc ? <p className="mx-auto mt-1 max-w-md text-xs leading-relaxed text-slate-500">{desc}</p> : null}
      </div>
      {action}
    </div>
  )
}

export function ConfirmDialog({
  open,
  title,
  message,
  confirmLabel,
  cancelLabel,
  danger = false,
  busy = false,
  layer = LAYERS.top,
  onConfirm,
  onCancel,
}) {
  if (!open) return null
  return (
    <div className="fixed inset-0 flex items-center justify-center" style={{ zIndex: layer }}>
      <div className="absolute inset-0 bg-slate-900/25" onClick={onCancel} />
      <div className="relative w-[400px] max-w-[90vw] rounded-xl bg-white p-5 shadow-2xl">
        <h3 className="text-sm font-semibold text-slate-800">{title}</h3>
        {message ? <p className="mt-2 text-xs leading-relaxed text-slate-500">{message}</p> : null}
        <div className="mt-5 flex justify-end gap-2">
          <Button variant="ghost" onClick={onCancel} disabled={busy}>
            {cancelLabel}
          </Button>
          <Button variant={danger ? 'danger' : 'primary'} onClick={onConfirm} disabled={busy}>
            {confirmLabel}
          </Button>
        </div>
      </div>
    </div>
  )
}
