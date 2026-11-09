package mqtt_consume

import (
	"os"
	"path/filepath"
	"strings"
)

// expandHomePath 将路径前缀 ~ 展开为用户主目录（跨平台：Linux 为 $HOME，Windows 为 %USERPROFILE%）。
// 展开失败时原样返回，由后续文件读取报错。
func expandHomePath(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~\\") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(p, "~"), string(filepath.Separator)))
		}
	}
	return p
}
