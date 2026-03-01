# trace-file

File driver for `github.com/infrago/trace`.

## Install

```go
import _ "github.com/infrago/trace-file"
```

## Config

```toml
[trace.file]
driver = "file"
json = true
fields = { trace_id = "tid", span_id = "sid", parent_span_id = "psid", timestamp = "ts" }

[trace.file.setting]
store = "store/trace"
output = "trace.log"
maxsize = "100MB"
slice = "day"
maxline = 0
compress = true
maxage = "7d"
maxfiles = 30
```

- `store`: base directory for relative output path
- `output|file|path`: output file path
- `maxsize`: rotate when size exceeds limit
- `slice`: rotate by time window (`year|month|day|hour`)
- `maxline`: rotate by line count
- `compress`: gzip rotated files (`.gz`)
- `maxage`: remove rotated files older than age
- `maxfiles`: keep only latest rotated files
- `fields` (on `[trace.file]`): output fields selection / mapping (array or map)
