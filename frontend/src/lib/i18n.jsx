import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import { GetALLLang, GetCurrentLang, GetLangTextMap, SetLanguage } from '../../wailsjs/go/service/App'

const I18nContext = createContext({
  t: (key) => key,
  lang: 'zh-CN',
  langs: [],
  changeLang: async () => {},
})

/**
 * 语言包由后端统一管理（内嵌 + 磁盘目录，且已按默认语言做过回落），
 * 前端只负责取值与切换。
 */
export function I18nProvider({ children }) {
  const [textMap, setTextMap] = useState({})
  const [lang, setLang] = useState('zh-CN')
  const [langs, setLangs] = useState([])

  const reload = useCallback(async () => {
    try {
      const [map, current, all] = await Promise.all([GetLangTextMap(), GetCurrentLang(), GetALLLang()])
      setTextMap(map || {})
      setLang(current || 'zh-CN')
      setLangs(all || [])
    } catch (err) {
      console.error('加载语言包失败', err)
    }
  }, [])

  useEffect(() => {
    reload()
  }, [reload])

  const t = useCallback(
    (key, fallback) => {
      const value = textMap[key]
      if (value !== undefined && value !== '') return value
      return fallback ?? key
    },
    [textMap],
  )

  const changeLang = useCallback(
    async (code) => {
      await SetLanguage(code)
      await reload()
    },
    [reload],
  )

  const value = useMemo(
    () => ({ t, lang, langs, changeLang, reload }),
    [t, lang, langs, changeLang, reload],
  )

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>
}

export function useI18n() {
  return useContext(I18nContext)
}
