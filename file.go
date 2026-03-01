package trace_file

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/infrago/infra"
	. "github.com/infrago/base"
	"github.com/infrago/trace"
)

type (
	fileDriver struct{}

	fileConnection struct {
		instance *trace.Instance
		setting  fileSetting
		writer   *rotatingWriter
	}

	fileSetting struct {
		store    string
		output   string
		maxSize  int64
		slice    string
		maxLine  int64
		compress bool
		maxAge   time.Duration
		maxFiles int
	}
)

func init() {
	infra.Register("file", &fileDriver{})
}

func (d *fileDriver) Connect(inst *trace.Instance) (trace.Connection, error) {
	setting := fileSetting{
		store:    "store/trace",
		output:   "trace.log",
		maxSize:  100 * 1024 * 1024,
		slice:    "",
		maxLine:  0,
		compress: false,
		maxAge:   0,
		maxFiles: 0,
	}

	if inst != nil {
		if v, ok := getString(inst.Setting, "store"); ok && v != "" {
			setting.store = v
		}
		if v, ok := getString(inst.Setting, "output"); ok && v != "" {
			setting.output = v
		}
		if v, ok := getString(inst.Setting, "file"); ok && v != "" {
			setting.output = v
		}
		if v, ok := getString(inst.Setting, "path"); ok && v != "" {
			setting.output = v
		}

		if v, ok := getString(inst.Setting, "maxsize"); ok && v != "" {
			if size, ok := parseSize(v); ok && size > 0 {
				setting.maxSize = size
			}
		}
		if v, ok := getInt64(inst.Setting, "maxsize"); ok && v > 0 {
			setting.maxSize = v
		}
		if v, ok := getString(inst.Setting, "slice"); ok {
			setting.slice = normalizeSlice(v)
		}
		if v, ok := getInt64(inst.Setting, "maxline"); ok && v > 0 {
			setting.maxLine = v
		}
		if v, ok := getBool(inst.Setting, "compress"); ok {
			setting.compress = v
		}
		if v, ok := getDuration(inst.Setting, "maxage"); ok && v > 0 {
			setting.maxAge = v
		}
		if v, ok := getInt(inst.Setting, "maxfiles"); ok && v > 0 {
			setting.maxFiles = v
		}
	}

	return &fileConnection{instance: inst, setting: setting}, nil
}

func (c *fileConnection) Open() error {
	path := c.resolvePath(c.setting.output)
	w, err := newRotatingWriter(path, c.setting.maxSize, c.setting.slice, c.setting.maxLine, c.setting.compress, c.setting.maxAge, c.setting.maxFiles)
	if err != nil {
		return err
	}
	c.writer = w
	return nil
}

func (c *fileConnection) Close() error {
	if c.writer == nil {
		return nil
	}
	return c.writer.Close()
}

func (c *fileConnection) Write(spans ...trace.Span) error {
	if c.writer == nil {
		return nil
	}
	for _, span := range spans {
		line := c.instance.Format(span)
		if err := c.writer.WriteLine(line); err != nil {
			return err
		}
	}
	return nil
}

func (c *fileConnection) resolvePath(file string) string {
	if filepath.IsAbs(file) {
		return file
	}
	return filepath.Join(c.setting.store, file)
}

func getString(m Map, key string) (string, bool) {
	if m == nil {
		return "", false
	}
	value, ok := m[key]
	if !ok {
		return "", false
	}
	v, ok := value.(string)
	return v, ok
}

func getInt64(m Map, key string) (int64, bool) {
	if m == nil {
		return 0, false
	}
	value, ok := m[key]
	if !ok {
		return 0, false
	}
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int64:
		return v, true
	case float64:
		return int64(v), true
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err == nil {
			return n, true
		}
	}
	return 0, false
}

func getInt(m Map, key string) (int, bool) {
	if v, ok := getInt64(m, key); ok {
		return int(v), true
	}
	return 0, false
}

func getBool(m Map, key string) (bool, bool) {
	if m == nil {
		return false, false
	}
	value, ok := m[key]
	if !ok {
		return false, false
	}
	v, ok := value.(bool)
	return v, ok
}

func getDuration(m Map, key string) (time.Duration, bool) {
	if m == nil {
		return 0, false
	}
	value, ok := m[key]
	if !ok {
		return 0, false
	}
	switch v := value.(type) {
	case time.Duration:
		return v, true
	case int:
		return time.Second * time.Duration(v), true
	case int64:
		return time.Second * time.Duration(v), true
	case float64:
		return time.Second * time.Duration(v), true
	case string:
		raw := strings.TrimSpace(v)
		d, err := time.ParseDuration(raw)
		if err == nil {
			return d, true
		}
		if strings.HasSuffix(strings.ToLower(raw), "d") {
			n, err := strconv.Atoi(strings.TrimSpace(raw[:len(raw)-1]))
			if err == nil && n > 0 {
				return time.Hour * 24 * time.Duration(n), true
			}
		}
	}
	return 0, false
}

func normalizeSlice(slice string) string {
	switch strings.ToLower(slice) {
	case "year", "y":
		return "year"
	case "month", "m":
		return "month"
	case "day", "d":
		return "day"
	case "hour", "h":
		return "hour"
	default:
		return ""
	}
}

func parseSize(raw string) (int64, bool) {
	value := strings.TrimSpace(strings.ToUpper(raw))
	if value == "" {
		return 0, false
	}

	units := []struct {
		suffix string
		scale  int64
	}{
		{"GB", 1024 * 1024 * 1024},
		{"G", 1024 * 1024 * 1024},
		{"MB", 1024 * 1024},
		{"M", 1024 * 1024},
		{"KB", 1024},
		{"K", 1024},
		{"B", 1},
	}

	for _, unit := range units {
		if strings.HasSuffix(value, unit.suffix) {
			number := strings.TrimSpace(strings.TrimSuffix(value, unit.suffix))
			if number == "" {
				return 0, false
			}
			f, err := strconv.ParseFloat(number, 64)
			if err != nil {
				return 0, false
			}
			return int64(f * float64(unit.scale)), true
		}
	}

	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

type rotatedMeta struct {
	path string
	ts   time.Time
}

func listRotatedFiles(filename string) ([]rotatedMeta, error) {
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	patterns := []string{
		fmt.Sprintf("%s.*%s", base, ext),
		fmt.Sprintf("%s.*%s.gz", base, ext),
	}
	seen := map[string]struct{}{}
	paths := make([]string, 0)
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		for _, path := range matches {
			if _, ok := seen[path]; ok {
				continue
			}
			seen[path] = struct{}{}
			paths = append(paths, path)
		}
	}
	files := make([]rotatedMeta, 0, len(paths))
	for _, path := range paths {
		ts := parseRotatedTimestamp(path, ext)
		if ts.IsZero() {
			if info, err := os.Stat(path); err == nil {
				ts = info.ModTime()
			}
		}
		files = append(files, rotatedMeta{path: path, ts: ts})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].ts.After(files[j].ts) })
	return files, nil
}

func parseRotatedTimestamp(path string, ext string) time.Time {
	name := filepath.Base(path)
	if strings.HasSuffix(name, ".gz") {
		name = strings.TrimSuffix(name, ".gz")
	}
	if ext != "" {
		name = strings.TrimSuffix(name, ext)
	}
	const layout = "20060102.150405"
	if len(name) < len(layout) {
		return time.Time{}
	}
	raw := name[len(name)-len(layout):]
	t, err := time.Parse(layout, raw)
	if err != nil {
		return time.Time{}
	}
	return t
}

func rotatedName(filename string, now time.Time) string {
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	return fmt.Sprintf("%s.%s%s", base, now.Format("20060102.150405"), ext)
}
