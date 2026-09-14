# -*- coding: utf-8 -*-
"""校验 package-lock.json 是否覆盖了所有构建平台的原生依赖。

背景：npm 有个老问题（npm/cli#4828）——在某个平台上跑 `npm install` 时，
只把**当前平台**的 optional 依赖写进 lockfile。于是 Windows 上生成的 lock
拿到 Linux/macOS 上 `npm ci`，rollup 的 native loader 会因为找不到
@rollup/rollup-linux-x64-gnu 之类而直接抛错。

这个脚本不硬编码包名，而是从依赖树自身推导：任何包的 optionalDependencies 里
声明了带平台名的可选包，lock 里就必须真的有那一条。这样将来新引入带原生二进制的
依赖（tailwindcss/oxide、lightningcss 之类）也会被自动覆盖。

用法：
  python scripts/check-lockfile.py                      # 校验 frontend/package-lock.json
  python scripts/check-lockfile.py <lockfile> [lock2]   # 校验指定文件
退出码：0 = 覆盖完整；1 = 缺平台条目
"""
import json
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
DEFAULT_LOCK = os.path.join(ROOT, "frontend", "package-lock.json")

# 会被 npm 用来区分平台的命名片段。
PLATFORM_HINT = re.compile(
    r"-(linux|darwin|win32|android|freebsd|openbsd|openharmony|sunos|netbsd|aix)"
)


def check(lock_path):
    with open(lock_path, encoding="utf-8") as f:
        lock = json.load(f)

    packages = lock.get("packages")
    if packages is None:
        print("  [!] 不是 lockfileVersion 2/3 格式（缺 packages 字段），无法校验")
        return 1

    missing = []
    checked_parents = 0
    checked_deps = 0

    for parent, entry in packages.items():
        optional = entry.get("optionalDependencies") or {}
        platform_deps = [d for d in optional if PLATFORM_HINT.search(d)]
        if not platform_deps:
            continue
        checked_parents += 1
        owner = parent.replace("node_modules/", "") or "(root)"
        for dep in platform_deps:
            checked_deps += 1
            if f"node_modules/{dep}" not in packages:
                missing.append((owner, dep))

    print(f"== {os.path.relpath(lock_path, ROOT)} ==")
    print(f"  含平台可选依赖的包：{checked_parents} 个，声明条目：{checked_deps} 条")
    print(f"  实际存在的平台二进制："
          f"{sum(1 for k in packages if PLATFORM_HINT.search(k))} 个")

    if missing:
        by_owner = {}
        for owner, dep in missing:
            by_owner.setdefault(owner, []).append(dep)
        print(f"  [!] 缺少 {len(missing)} 个平台条目：")
        for owner, deps in sorted(by_owner.items()):
            sample = ", ".join(sorted(deps)[:4])
            more = f" 等 {len(deps)} 个" if len(deps) > 4 else ""
            print(f"      {owner}: {sample}{more}")
        print()
        print("  原因：这份 lock 是在单个平台上生成的，其他平台的 optional 依赖没被写进去。")
        print("  修法：删掉 node_modules 与 package-lock.json，在有网络的机器上重新 `npm install`，")
        print("        然后把新的 lock 提交上来（切记别用 `--omit=optional` 或 `--no-optional`）。")
        return 1

    print("  lock 覆盖完整")
    return 0


def main():
    paths = sys.argv[1:] or [DEFAULT_LOCK]
    failed = 0
    for p in paths:
        if not os.path.isfile(p):
            print(f"  [!] 找不到 {p}")
            failed = 1
            continue
        if check(p) != 0:
            failed = 1
        print()
    if failed:
        print("结果：不通过")
    else:
        print("结果：通过（所有平台的原生依赖都在 lock 里）")
    return failed


if __name__ == "__main__":
    sys.exit(main())
