package webdav

import "strings"

// ResolvePath는 주어진 경로를 분석하여 space 이름과 파일 경로를 반환한다.
func ResolvePath(path string) (spaceName string, filePath string) {
	trimmed := strings.TrimPrefix(path, "/")

	// 3분할
	parts := strings.SplitN(trimmed, "/", 3)

	// dav, spaceName, filePath
	if len(parts) < 2 || parts[0] != "dav" {
		return "", ""
	}

	spaceName = parts[1]

	// 파일 경로가 없는 경우 루트로 설정
	if len(parts) > 2 {
		filePath = "/" + parts[2]
	} else {
		filePath = "/"
	}

	return spaceName, filePath
}
