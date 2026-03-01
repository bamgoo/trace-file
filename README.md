# trace-file

`trace-file` 是 `trace` 模块的 `file` 驱动。

## 安装

```bash
go get github.com/infrago/trace@latest
go get github.com/infrago/trace-file@latest
```

## 接入

```go
import (
    _ "github.com/infrago/trace"
    _ "github.com/infrago/trace-file"
    "github.com/infrago/infra"
)

func main() {
    infra.Run()
}
```

## 配置示例

```toml
[trace]
driver = "file"
```

## 公开 API（摘自源码）

- `func (w *rotatingWriter) Close() error`
- `func (w *rotatingWriter) WriteLine(line string) error`
- `func (d *fileDriver) Connect(inst *trace.Instance) (trace.Connection, error)`
- `func (c *fileConnection) Open() error`
- `func (c *fileConnection) Close() error`
- `func (c *fileConnection) Write(spans ...trace.Span) error`

## 排错

- driver 未生效：确认模块段 `driver` 值与驱动名一致
- 连接失败：检查 endpoint/host/port/鉴权配置
