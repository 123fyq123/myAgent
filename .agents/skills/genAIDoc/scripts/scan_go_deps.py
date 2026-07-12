"""扫描 Go 模块依赖并输出 JSON。

用途：
- 为 Go 模块的 `AI_CONTEXT.md` 生成提供依赖事实。
- 从 `go.mod` 读取 require / replace。
- 从 Go import 中识别内部模块依赖、外部依赖、标准库依赖和反向依赖。

输出：
- 向标准输出写入 JSON，不直接修改项目文件。
"""

import argparse
import json
import re
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


def is_excluded(path: Path, root: Path) -> bool:
    try:
        relative = path.relative_to(root)
    except ValueError:
        return False
    return any(part in EXCLUDE_NAMES for part in relative.parts)


def rel(path: Path, root: Path) -> str:
    return str(path.relative_to(root)).replace("/", "\\")


def strip_line_comment(line: str) -> str:
    return re.sub(r"//.*$", "", line).strip()


def read_go_module(go_mod: Path) -> str | None:
    for line in go_mod.read_text(encoding="utf-8", errors="ignore").splitlines():
        stripped = line.strip()
        if stripped.startswith("module "):
            return stripped.split(maxsplit=1)[1].strip()
    return None


def parse_go_mod_requires(go_mod: Path) -> list[str]:
    requires: set[str] = set()
    in_block = False
    for raw_line in go_mod.read_text(encoding="utf-8", errors="ignore").splitlines():
        line = strip_line_comment(raw_line)
        if line == "require (":
            in_block = True
            continue
        if in_block and line == ")":
            in_block = False
            continue
        if in_block:
            parts = line.split()
            if parts:
                requires.add(parts[0])
            continue
        if line.startswith("require "):
            parts = line.split()
            if len(parts) >= 2:
                requires.add(parts[1])
    return sorted(requires)


def parse_go_mod_replaces(go_mod: Path) -> list[dict]:
    replaces = []
    in_block = False
    pattern = re.compile(r"^(\S+)(?:\s+\S+)?\s+=>\s+(\S+)")
    for raw_line in go_mod.read_text(encoding="utf-8", errors="ignore").splitlines():
        line = strip_line_comment(raw_line)
        if line == "replace (":
            in_block = True
            continue
        if in_block and line == ")":
            in_block = False
            continue
        candidate = line
        if not in_block and candidate.startswith("replace "):
            candidate = candidate.removeprefix("replace ").strip()
        if in_block or line.startswith("replace "):
            match = pattern.match(candidate)
            if match:
                replaces.append({"old": match.group(1), "new": match.group(2)})
    return replaces


def parse_go_imports(module_dir: Path, root: Path) -> list[str]:
    imports: set[str] = set()
    single_pattern = re.compile(r'^import\s+(?:[._A-Za-z0-9]+\s+)?"([^"]+)"')
    block_pattern = re.compile(r'^(?:[._A-Za-z0-9]+\s+)?"([^"]+)"')

    for go_file in module_dir.rglob("*.go"):
        if is_excluded(go_file.parent, root):
            continue
        in_block = False
        for raw_line in go_file.read_text(encoding="utf-8", errors="ignore").splitlines():
            line = strip_line_comment(raw_line)
            if line == "import (":
                in_block = True
                continue
            if in_block and line == ")":
                in_block = False
                continue
            if in_block:
                match = block_pattern.match(line)
                if match:
                    imports.add(match.group(1))
                continue
            match = single_pattern.match(line)
            if match:
                imports.add(match.group(1))
    return sorted(imports)


def is_stdlib_import(import_path: str) -> bool:
    first = import_path.split("/", 1)[0]
    return "." not in first


def collect_modules(root: Path) -> list[dict]:
    modules = []
    for go_mod in root.rglob("go.mod"):
        if is_excluded(go_mod.parent, root):
            continue
        modules.append(
            {
                "dir": go_mod.parent,
                "path": rel(go_mod.parent, root),
                "module": read_go_module(go_mod),
                "go_mod": rel(go_mod, root),
            }
        )
    return sorted(modules, key=lambda item: item["path"])


def scan_go_deps(root: Path, module_path: str | None) -> dict:
    all_modules = collect_modules(root)
    module_defs = all_modules
    if module_path:
        target = (root / module_path).resolve()
        module_defs = [module for module in all_modules if module["dir"] == target]

    all_module_names = [module["module"] for module in all_modules if module["module"]]
    all_module_imports = {
        module["module"]: parse_go_imports(module["dir"], root)
        for module in all_modules
        if module["module"]
    }
    results = []

    for module in module_defs:
        go_mod = module["dir"] / "go.mod"
        internal_modules: set[str] = set()
        internal_packages: set[str] = set()
        external: set[str] = set()
        stdlib: set[str] = set()

        for import_path in parse_go_imports(module["dir"], root):
            matched = False
            for module_name in all_module_names:
                if import_path == module_name or import_path.startswith(module_name + "/"):
                    internal_modules.add(module_name)
                    internal_packages.add(import_path)
                    matched = True
                    break
            if not matched:
                if is_stdlib_import(import_path):
                    stdlib.add(import_path)
                else:
                    external.add(import_path)

        results.append(
            {
                "path": module["path"],
                "module": module["module"],
                "go_mod": module["go_mod"],
                "requires": parse_go_mod_requires(go_mod),
                "replaces": parse_go_mod_replaces(go_mod),
                "imports_internal_modules": sorted(internal_modules),
                "imports_internal_packages": sorted(internal_packages),
                "imports_external": sorted(external),
                "imports_stdlib": sorted(stdlib),
                "dependents": [],
            }
        )

    for target in results:
        dependents = []
        for candidate_module, candidate_imports in all_module_imports.items():
            if candidate_module == target["module"]:
                continue
            if any(
                import_path == target["module"] or import_path.startswith(target["module"] + "/")
                for import_path in candidate_imports
            ):
                dependents.append(candidate_module)
        target["dependents"] = sorted(dependents)

    return {
        "root": str(root),
        "generated_at": datetime.now(timezone.utc).astimezone().isoformat(),
        "modules": sorted(results, key=lambda item: item["path"]),
    }


def main() -> None:
    parser = argparse.ArgumentParser(description="扫描 Go 模块依赖并输出 JSON。")
    parser.add_argument("--root", default=".", help="Go 工作区或仓库根目录。")
    parser.add_argument("--module-path", default="", help="只扫描指定模块路径。")
    args = parser.parse_args()

    root = Path(args.root).resolve()
    module_path = args.module_path.strip() or None
    print(json.dumps(scan_go_deps(root, module_path), ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
