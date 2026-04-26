# AGENTS.md

## License Header 规范

修改源代码文件时，必须同步更新文件头部的 copyright 年份标注。

### 规则

- 如果文件原始 copyright 年份为 `YYYY`，且当前修改年份不同，更新为 `YYYY-当前年份`
- 如果已经是范围格式 `YYYY-ZZZZ`，将结束年份更新为当前年份
- 仅修改本次 PR 实际变更的文件，不要批量更新未修改的文件

### 示例

```
// 原始
// Copyright 2025 Matrix Origin

// 修改后（当前年份 2026）
// Copyright 2025-2026 Matrix Origin
```
