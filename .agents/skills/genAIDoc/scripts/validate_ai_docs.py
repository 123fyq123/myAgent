"""校验 AI 文档中的占位符和引用路径。

用途：
- 在生成或更新 `AI_INDEX.md` / `AI_CONTEXT.md` 后执行质量检查。
- 检查常见占位文本。
- 检查指向 AI 文档的相对路径和 Markdown 链接是否存在。

输出：
- 向标准输出写入 JSON。
- 发现问题时以非 0 状态码退出。
"""

import argparse
import json
import re
import sys
from datetime import datetime, timezone
from pathlib import Path


PLACEHOLDER_PATTERN = re.compile(r"TODO|FIXME|TBD|待填写|待补充|请补充|xxx|\.\.\.")
DOC_NAME_PATTERN = re.compile(r"(AI_CONTEXT|AI_INDEX)\.md")


def rel(path: Path, root: Path) -> str:
    return str(path.relative_to(root)).replace("/", "\\")


def reference_exists(doc_dir: Path, reference: str) -> bool:
    if re.match(r"^[a-zA-Z]+://", reference):
        return True
    ref_path = Path(reference)
    if ref_path.is_absolute():
        return ref_path.exists()
    return (doc_dir / ref_path).exists()


def add_issue(issues: list[dict], level: str, file: str, line: int, message: str) -> None:
    issues.append({"level": level, "file": file, "line": line, "message": message})


def validate(root: Path) -> tuple[dict, int]:
    issues: list[dict] = []
    docs = sorted(
        [path for path in root.rglob("*") if path.is_file() and path.name in {"AI_INDEX.md", "AI_CONTEXT.md"}],
        key=lambda path: rel(path, root),
    )

    for doc in docs:
        relative_doc = rel(doc, root)
        doc_dir = doc.parent
        lines = doc.read_text(encoding="utf-8", errors="ignore").splitlines()
        for index, line in enumerate(lines, start=1):
            if PLACEHOLDER_PATTERN.search(line):
                add_issue(issues, "error", relative_doc, index, "发现占位文本")

            for match in re.finditer(r"`([^`]*(?:AI_CONTEXT|AI_INDEX)\.md)`", line):
                reference = match.group(1)
                if reference in {"AI_CONTEXT.md", "AI_INDEX.md"}:
                    continue
                if not reference_exists(doc_dir, reference):
                    add_issue(issues, "error", relative_doc, index, f"文档引用路径不存在: {reference}")

            for match in re.finditer(r"\]\(([^)]+)\)", line):
                reference = match.group(1)
                if DOC_NAME_PATTERN.search(reference) and not reference_exists(doc_dir, reference):
                    add_issue(issues, "error", relative_doc, index, f"Markdown 链接目标不存在: {reference}")

    for index_doc in [doc for doc in docs if doc.name == "AI_INDEX.md"]:
        content = index_doc.read_text(encoding="utf-8", errors="ignore")
        if "AI_CONTEXT.md" not in content:
            add_issue(issues, "warning", rel(index_doc, root), 0, "AI_INDEX.md 未引用任何 AI_CONTEXT.md")

    result = {
        "root": str(root),
        "generated_at": datetime.now(timezone.utc).astimezone().isoformat(),
        "checked_files": [rel(doc, root) for doc in docs],
        "issue_count": len(issues),
        "issues": issues,
    }
    return result, 1 if issues else 0


def main() -> None:
    parser = argparse.ArgumentParser(description="校验 AI 文档占位符和引用路径。")
    parser.add_argument("--root", default=".", help="仓库或工作区根目录。")
    args = parser.parse_args()

    result, exit_code = validate(Path(args.root).resolve())
    print(json.dumps(result, ensure_ascii=False, indent=2))
    sys.exit(exit_code)


if __name__ == "__main__":
    main()
