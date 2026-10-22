#!/usr/bin/env python3
"""qa_step_scan_3_9.py — scan-гигиена диффа story 3-9 (шаги 01-05).

Паттерны дисциплины 3-6/3-7 QA: секреты (12 паттернов), CJK-символы,
контрольные символы, длинные hex-строки (кандидаты секретов/SHA —
классифицируются вручную). Использование:
    python3 scripts/qa_step_scan_3_9.py            # скан диффа рабочей области (unstaged+untracked)
    python3 scripts/qa_step_scan_3_9.py <base>     # скан диффа от base-коммита
"""
import re
import subprocess
import sys

SECRET_PATTERNS = [
    r"github_pat_[A-Za-z0-9_]{20,}",
    r"ghp_[A-Za-z0-9]{30,}",
    r"gho_[A-Za-z0-9]{30,}",
    r"ghu_[A-Za-z0-9]{30,}",
    r"ghs_[A-Za-z0-9]{30,}",
    r"-----BEGIN [A-Z ]*PRIVATE KEY-----",
    r"AKIA[0-9A-Z]{16}",
    r"sk-[A-Za-z0-9]{20,}",
    r"xox[baprs]-[A-Za-z0-9-]{10,}",
    r"AIza[0-9A-Za-z_-]{35}",
    r"password\s*=\s*['\"][^'\"]{8,}['\"]",
    r"Bearer\s+[A-Za-z0-9._-]{20,}",
]
CJK = re.compile(r"[\u4e00-\u9fff\u3040-\u30ff\uac00-\ud7af]")
CTRL = re.compile(r"[\x00-\x08\x0b\x0c\x0e-\x1f]")
LONG_HEX = re.compile(r"\b[0-9a-f]{40,}\b")


def diff_files(base):
    if base:
        out = subprocess.run(
            ["git", "diff", "--name-only", base], capture_output=True, text=True, check=True
        ).stdout.splitlines()
    else:
        out = subprocess.run(
            ["git", "status", "--porcelain"], capture_output=True, text=True, check=True
        ).stdout.splitlines()
        return [line[3:].strip() for line in out if line[3:].strip()]
    return out


def diff_lines(base, path):
    if base:
        cmd = ["git", "diff", base, "--", path]
    else:
        cmd = ["git", "diff", "--", path]
    out = subprocess.run(cmd, capture_output=True, text=True, check=True).stdout
    added = [l[1:] for l in out.splitlines() if l.startswith("+") and not l.startswith("+++")]
    # untracked файлы: дифф пуст — читаем целиком как добавленное
    if not added and base is None:
        try:
            with open(path, encoding="utf-8") as fh:
                added = fh.read().splitlines()
        except (FileNotFoundError, UnicodeDecodeError):
            return []
    return added


def main():
    base = sys.argv[1] if len(sys.argv) > 1 else None
    files = diff_files(base)
    secrets, cjk, ctrl, hexes = [], [], [], []
    total_added = 0
    for path in files:
        lines = diff_lines(base, path)
        total_added += len(lines)
        for no, line in enumerate(lines, 1):
            for pat in SECRET_PATTERNS:
                if re.search(pat, line):
                    secrets.append(f"{path}:{no}: {pat}")
            if CJK.search(line):
                cjk.append(f"{path}:{no}")
            if CTRL.search(line):
                ctrl.append(f"{path}:{no}")
            for m in LONG_HEX.finditer(line):
                hexes.append(f"{path}:{no}: len={len(m.group(0))}")
    print(f"files={len(files)} added_lines={total_added}")
    print(f"secrets={len(secrets)}")
    for s in secrets:
        print(f"  SECRET {s}")
    print(f"cjk={len(cjk)}")
    for s in cjk:
        print(f"  CJK {s}")
    print(f"ctrl={len(ctrl)}")
    for s in ctrl:
        print(f"  CTRL {s}")
    print(f"long_hex_candidates={len(hexes)}")
    for s in hexes:
        print(f"  HEX {s}")
    verdict = "CLEAN" if not secrets and not cjk and not ctrl else "DIRTY"
    print(f"VERDICT: {verdict}" + ("" if verdict == "CLEAN" else " — разбери находки выше"))
    return 0 if verdict == "CLEAN" else 1


if __name__ == "__main__":
    sys.exit(main())
