package buildinfo

import "strings"

var (
	Version   = "development"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

type Info struct {
	Version   string
	GitCommit string
	BuildTime string
}

// Current 返回链接阶段注入的非敏感构建元数据。
func Current() Info {
	return Info{
		Version:   valueOrDefault(Version, "development"),
		GitCommit: valueOrDefault(GitCommit, "unknown"),
		BuildTime: valueOrDefault(BuildTime, "unknown"),
	}
}

func valueOrDefault(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
