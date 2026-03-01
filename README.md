# trace-file

`trace-file` 是 `github.com/infrago/trace` 的**file 驱动**。

## 包定位

- 类型：驱动
- 作用：把 `trace` 模块的统一接口落到 `file` 后端实现

## 快速接入

```go
import (
    _ "github.com/infrago/trace"
    _ "github.com/infrago/trace-file"
)
```

```toml
[trace]
driver = "file"
```

## `setting` 专用配置项

配置位置：`[trace].setting`

- `store`
- `output`
- `file`
- `path`
- `maxsize`
- `slice`
- `maxline`
- `compress`
- `maxage`
- `maxfiles`

## 说明

- `setting` 仅对当前驱动生效，不同驱动键名可能不同
- 连接失败时优先核对 `setting` 中 host/port/认证/超时等参数
