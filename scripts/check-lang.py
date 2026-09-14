# -*- coding: utf-8 -*-
"""语言包完整性校验。

覆盖四件事：
  1. 各语言 textmap 与 default 互相对齐（缺失 / 多余 / 空值 / 占位符）
  2. key 的两端引用反查：前端 t('k') 与后端 global.T("k")，
     找出「用了但没有」与「谁都没用」
  3. 漏网之鱼：源码里写死的中文文案（既不走 t() 也不走 global.T()）
  4. 译文真实性抽查与 info.json 完整性

用法：python scripts/check-lang.py
退出码：0 = 通过；1 = 存在问题
"""
import json
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
LANG_DIR = os.path.join(ROOT, "lang")
SRC_DIR = os.path.join(ROOT, "frontend", "src")
GO_DIRS = ["api", "plugin", "service", "store", "upstream", "backup", "global"]

# 前端通过变量间接调用的 key（如 t(meta.key) / t(mode ? a : b)），
# 直接字面量扫描抓不到，列出来避免误报为「没人用」。
INDIRECT_KEYS = {
    "probe_ok", "probe_auth", "probe_fail", "probe_error", "probe_unknown",
    "cap_tool_call", "cap_images", "cap_reasoning",
    "cache_group_empty", "agent_group_empty",
}

# 后端文案的 key 前缀：前端拿到的是已经本地化好的字符串，不会出现这些 key。
BACKEND_PREFIXES = ("err_", "probe_msg_")

# 语言系统自身的诊断信息刻意不本地化：报错的时候正是语言包可能读不出来的
# 时候，用语言包来渲染这些错误会自我依赖。
I18N_ALLOWED = {
    "嵌入的文件系统未初始化",
    "解析内嵌 info.json 失败",
    "解析内嵌 textmap.json 失败",
    "解析 info.json 失败",
    "解析 textmap.json 失败",
}

CJK = re.compile(r"[\u4e00-\u9fff]")
PLACEHOLDER = re.compile(r"\{\{?\s*([a-zA-Z0-9_]+)\s*\}?\}")
JS_CALL = re.compile(r"""\bt\(\s*['"]([a-zA-Z0-9_.]+)['"]""")
GO_CALL = re.compile(r"""global\.T\(\s*"([a-zA-Z0-9_.]+)""")
JS_STRING = re.compile(r"""['"`]([^'"`\n]*[\u4e00-\u9fff][^'"`\n]*)['"`]""")

problems = []


def fail(msg):
    problems.append(msg)
    print("  [!] " + msg)


def load_packs():
    packs = {}
    for name in sorted(os.listdir(LANG_DIR)):
        path = os.path.join(LANG_DIR, name, "textmap.json")
        if os.path.isfile(path):
            with open(path, encoding="utf-8") as f:
                packs[name] = json.load(f)
    return packs


def strip_comments(src):
    src = re.sub(r"/\*.*?\*/", "", src, flags=re.S)
    src = re.sub(r"^[ \t]*//.*$", "", src, flags=re.M)
    return re.sub(r"//[^\n]*$", "", src, flags=re.M)


def used_keys():
    """返回 (前端引用的 key, 后端引用的 key)。"""
    front, back = {}, {}
    for dirpath, _, files in os.walk(SRC_DIR):
        for fn in files:
            if not fn.endswith((".jsx", ".js", ".tsx", ".ts")):
                continue
            fp = os.path.join(dirpath, fn)
            with open(fp, encoding="utf-8") as f:
                for key in JS_CALL.findall(f.read()):
                    front.setdefault(key, set()).add(os.path.relpath(fp, ROOT))
    for d in GO_DIRS:
        full = os.path.join(ROOT, d)
        if not os.path.isdir(full):
            continue
        for fn in sorted(os.listdir(full)):
            if not fn.endswith(".go"):
                continue
            fp = os.path.join(full, fn)
            with open(fp, encoding="utf-8") as f:
                for key in GO_CALL.findall(f.read()):
                    back.setdefault(key, set()).add(f"{d}/{fn}")
    return front, back


def hardcoded_strings():
    """源码里写死的中文文案（保留 console.* 与测试夹具不算）。"""
    found = []
    for dirpath, _, files in os.walk(SRC_DIR):
        for fn in files:
            if not fn.endswith((".jsx", ".js")):
                continue
            fp = os.path.join(dirpath, fn)
            with open(fp, encoding="utf-8") as f:
                lines = strip_comments(f.read()).splitlines()
            for i, line in enumerate(lines, 1):
                if line.strip().startswith(("console.", "//")):
                    continue
                for m in JS_STRING.finditer(line):
                    found.append((os.path.relpath(fp, ROOT), i, m.group(1)))
    # 后端只保留「日志以外」的检查：日志文案固定中文是有意为之。
    for d in GO_DIRS:
        full = os.path.join(ROOT, d)
        if not os.path.isdir(full):
            continue
        for fn in sorted(os.listdir(full)):
            if not fn.endswith(".go") or fn.endswith("_test.go"):
                continue
            fp = os.path.join(full, fn)
            with open(fp, encoding="utf-8") as f:
                for i, line in enumerate(f, 1):
                    if not re.search(r"(Errorf|errors\.New|fail)\(", line):
                        continue
                    if re.search(r"Log\.(Warn|Error|Info|Debug)", line):
                        continue
                for m in JS_STRING.finditer(line):
                    if any(m.group(1).startswith(ok) for ok in I18N_ALLOWED):
                        continue
                    found.append((f"{d}/{fn}", i, m.group(1)))
    return found


def main():
    packs = load_packs()
    base = packs.get("default")
    if not base:
        print("找不到 lang/default/textmap.json")
        return 1

    print("== 语言包规模 ==")
    for name, pack in packs.items():
        blank = [k for k, v in pack.items() if not (isinstance(v, str) and v.strip())]
        print("  %-10s %4d 键   空值 %d" % (name, len(pack), len(blank)))
        for k in blank:
            fail(f"{name} 有空值键: {k}")

    print("\n== 相对 default 的缺失 / 多余 ==")
    for name, pack in packs.items():
        if name == "default":
            continue
        missing = sorted(set(base) - set(pack))
        extra = sorted(set(pack) - set(base))
        print("  %-10s 缺失 %d  多余 %d" % (name, len(missing), len(extra)))
        for k in missing:
            fail(f"{name} 缺键: {k}")
        for k in extra:
            fail(f"{name} 多键: {k}")

    print("\n== 占位符一致性 ==")
    # 只比「占位符个数」：同一条文案在不同语言里语序不同，用 %[2]s 这类
    # 显式下标（Go 的写法）是完全合法的，跟 %s 不能算不一致。
    verb = re.compile(r"%(\[\d+\])?[sdvq]")
    bad = 0
    for key, value in base.items():
        if not isinstance(value, str):
            continue
        expect = len(verb.findall(value)) or len(PLACEHOLDER.findall(value))
        if not expect:
            continue
        for name, pack in packs.items():
            if name == "default":
                continue
            other = pack.get(key)
            if not isinstance(other, str):
                continue
            got = len(verb.findall(other)) or len(PLACEHOLDER.findall(other))
            if got != expect:
                bad += 1
                fail(f"{name} / {key} 占位符个数不一致: default={expect} vs {got}")
    if bad == 0:
        print("  无不一致")

    front, back = used_keys()
    print(f"\n== 引用反查（前端 {len(front)} 个 key，后端 {len(back)} 个 key）==")
    referenced = set(front) | set(back)
    missing_in_lang = sorted(k for k in referenced if k not in base)
    for k in missing_in_lang:
        where = ", ".join(sorted(front.get(k, back.get(k, set()))))
        fail(f"用了但语言包里没有: {k}  <- {where}")
    if not missing_in_lang:
        print("  所有被引用的 key 都存在")

    dead = sorted(
        k for k in base
        if k not in referenced
        and k not in INDIRECT_KEYS
        and not k.startswith(BACKEND_PREFIXES)
    )
    for k in dead:
        fail(f"疑似死键（谁都没引用）: {k}")
    if not dead:
        print("  无死键")

    for name, pack in packs.items():
        miss = sorted(k for k in referenced if k not in pack)
        if miss:
            fail(f"{name} 缺少被引用的 key: {', '.join(miss)}")

    print("\n== 漏网之鱼：源码里写死的中文文案 ==")
    hard = hardcoded_strings()
    for path, line, text in hard:
        fail(f"{path}:{line}  {text}")
    if not hard:
        print("  无")

    print("\n== 译文真实性抽查（与中文逐字相同的键）==")
    for name, pack in packs.items():
        if name == "default":
            continue
        same = [k for k, v in pack.items() if v == base[k]]
        print("  %-10s 与中文完全相同 %d 处: %s" % (name, len(same), ", ".join(same) or "—"))

    print("\n== info.json ==")
    for name in sorted(os.listdir(LANG_DIR)):
        path = os.path.join(LANG_DIR, name, "info.json")
        if not os.path.isfile(path):
            fail(f"{name} 缺少 info.json")
            continue
        with open(path, encoding="utf-8") as f:
            info = json.load(f)
        for field in ("language_name", "language_code", "textmap_path"):
            if not info.get(field):
                fail(f"{name}/info.json 缺 {field}")
        print("  %-10s %s (%s)" % (name, info.get("language_name"), info.get("language_code")))

    print("\n" + ("通过：语言包完整，无漏网文案" if not problems else f"发现 {len(problems)} 处问题"))
    return 0 if not problems else 1


if __name__ == "__main__":
    sys.exit(main())
