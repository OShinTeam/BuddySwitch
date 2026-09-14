import { createContext, useCallback, useContext, useMemo, useState } from 'react'
import { Icons, LAYERS } from './ui'

const ToastContext = createContext({ toast: () => {}, success: () => {}, failure: () => {} })

const TONES = {
  info: { ring: 'ring-slate-200', icon: 'text-slate-400', iconEl: Icons.layers },
  success: { ring: 'ring-emerald-200', icon: 'text-emerald-500', iconEl: Icons.check },
  error: { ring: 'ring-rose-200', icon: 'text-rose-500', iconEl: Icons.alert },
}

/** 极简 toast：右下角堆叠，几秒后自动消失。 */
export function ToastProvider({ children }) {
  const [items, setItems] = useState([])

  const push = useCallback((tone, message, detail) => {
    const id = Math.random().toString(36).slice(2)
    setItems((prev) => [...prev, { id, tone, message, detail }])
    setTimeout(() => setItems((prev) => prev.filter((it) => it.id !== id)), tone === 'error' ? 6000 : 3000)
  }, [])

  const value = useMemo(
    () => ({
      toast: (message, detail) => push('info', message, detail),
      success: (message, detail) => push('success', message, detail),
      failure: (message, detail) => push('error', message, detail),
    }),
    [push],
  )

  return (
    <ToastContext.Provider value={value}>
      {children}
      {/* 提示要盖在所有浮层之上——对话框开着的时候也得看得见操作结果 */}
      <div
        className="pointer-events-none fixed bottom-4 right-4 flex w-80 flex-col gap-2"
        style={{ zIndex: LAYERS.top + 10 }}
      >
        {items.map((item) => {
          const tone = TONES[item.tone] || TONES.info
          const IconEl = tone.iconEl
          return (
            <div
              key={item.id}
              className={`pointer-events-auto flex items-start gap-2.5 rounded-lg bg-white px-3.5 py-2.5 shadow-lg ring-1 ${tone.ring}`}
            >
              <IconEl className={`mt-0.5 h-4 w-4 shrink-0 ${tone.icon}`} />
              <div className="min-w-0">
                <p className="text-[13px] font-medium text-slate-800">{item.message}</p>
                {item.detail ? (
                  <p className="mt-0.5 break-words text-[11px] leading-relaxed text-slate-500">{item.detail}</p>
                ) : null}
              </div>
            </div>
          )
        })}
      </div>
    </ToastContext.Provider>
  )
}

export function useToast() {
  return useContext(ToastContext)
}
