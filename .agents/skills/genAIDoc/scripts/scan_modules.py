"""扫描项目模块边界并输出 JSON。

用途：
- 为生成 `AI_INDEX.md` 提供模块清单。
- 为生成 `AI_CONTEXT.md` 前定位目标模块提供依据。
- 识别 Go 工作区、Go 模块、前端模块、服务入口和已有 AI 文档。

输出：
- 向标准输出写入 JSON，不直接修改项目文件。
"""

import argparse
import json
from datetime import datetime, timezone
from pathlib import Path


EXCLUDE_NAMES = {
    ".git",
    "node_modules",
    "vendor",
    "logs",
    ".gocache",
    "upload",
    "dist",
    ".idea",
    ".vscode",
    "profile_test",
    "analyze",
    "build",
    "curl",
    "t",
}

MARKER_FILES = {"go.mod", "go.work", "package.json", "main.go", "AI_CONTEXT.md"}


def is_excluded(path: Path, root: Path) -> bool:
    try:
        relative = path.relative_to(root)
    except ValueError:
        return False
    return any(part in EXCLUDE_NAMES for part in relative.parts)


def rel(path: Path, root: Path) -> str:
    return str(path.relative_to(root)).replace("/", "\\")


def read_go_module(go_mod: Path) -> str | None:
    for line in go_mod.read_text(encoding="utf-8", errors="ignore").splitlines():
        stripped = line.strip()
        if stripped.startswith("module "):
            return stripped.split(maxsplit=1)[1].strip()
    return None


def read_package_name(package_json: Path) -> str | None:
    try:
        data = json.loads(package_json.read_text(encoding="utf-8"))
    except Exception:
        return None
    return data.get("name")


def ensure_module(modules: dict[Path, dict], directory: Path, root: Path) -> dict:
    if directory not in modules:
        modules[directory] = {
            "path": rel(directory, root),
            "absolute_path": str(directory),
            "name": directory.name,
            "type": "unknown",
            "has_go_mod": False,
            "go_module": None,
            "has_go_work": False,
            "has_package_json": False,
            "package_name": None,
            "has_main_go": False,
            "has_ai_context": False,
            "ai_context_path": None,
        }
    return modules[directory]


def classify(module: dict) -> str:
    if module["has_go_work"]:
        return "go-workspace"
    if module["has_package_json"]:
        return "frontend"
    if module["has_go_mod"] and module["has_main_go"]:
        return "go-service"
    if module["has_go_mod"]:
        return "go-library"
    if module["has_main_go"]:
        return "go-entrypoint"
    return "unknown"


def scan_modules(root: Path) -> dict:
    modules: dict[Path, dict] = {}

    for path in root.rglob("*"):
        if not path.is_file() or path.name not in MARKER_FILES:
            continue
        if is_excluded(path.parent, root):
            continue

        module = ensure_module(modules, path.parent, root)
        if path.name == "go.mod":
            module["has_go_mod"] = True
            module["go_module"] = read_go_module(path)
        elif path.name == "go.work":
            module["has_go_work"] = True
        elif path.name == "package.json":
            module["has_package_json"] = True
            module["package_name"] = read_package_name(path)
        elif path.name == "main.go":
            module["has_main_go"] = True
        elif path.name == "AI_CONTEXT.md":
            module["has_ai_context"] = True
            module["ai_context_path"] = rel(path, root)

    result_modules = []
    for module in modules.values():
        module["type"] = classify(module)
        result_modules.append(module)

    return {
        "root": str(root),
        "generated_at": datetime.now(timezone.utc).astimezone().isoformat(),
        "modules": sorted(result_modules, key=lambda item: item["path"]),
    }


def main() -> None:
    parser = argparse.ArgumentParser(description="扫描项目模块边界并输出 JSON。")
    parser.add_argument("--root", default=".", help="仓库或工作区根目录。")
    args = parser.parse_args()

    root = Path(args.root).resolve()
    print(json.dumps(scan_modules(root), ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
