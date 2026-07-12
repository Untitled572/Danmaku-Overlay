package utils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// GenerateEpisodeID 生成 Episode ID
// 格式: {bangumiID}_{epIndex(3位)}_{验证位(1位)}
// 示例: 123_001_4 (bangumiID=123, epIndex=1, 验证位=4)
// 验证位算法: epIndex % 10 (O(1))
func GenerateEpisodeID(bangumiID uint, epIndex int) string {
	checkDigit := epIndex % 10
	return fmt.Sprintf("%d_%03d_%d", bangumiID, epIndex, checkDigit)
}

// ValidateEpisodeID 验证 Episode ID 的校验位
func ValidateEpisodeID(episodeID string) bool {
	parts := strings.Split(episodeID, "_")
	if len(parts) != 3 {
		return false
	}

	// 提取验证位
	checkDigit, err := strconv.Atoi(parts[2])
	if err != nil {
		return false
	}

	// 提取 epIndex
	epIndex, err := strconv.Atoi(parts[1])
	if err != nil {
		return false
	}

	return epIndex%10 == checkDigit
}

// ParseEpisodeID 解析 Episode ID 获取 bangumiID 和 epIndex
func ParseEpisodeID(episodeID string) (bangumiID uint, epIndex int, err error) {
	parts := strings.Split(episodeID, "_")
	if len(parts) != 3 {
		return 0, 0, fmt.Errorf("invalid episode ID format: %s", episodeID)
	}

	// 验证校验位
	if !ValidateEpisodeID(episodeID) {
		return 0, 0, fmt.Errorf("invalid checksum in episode ID: %s", episodeID)
	}

	// 提取 bangumiID
	bangumiID64, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse bangumiID: %w", err)
	}

	// 提取 epIndex
	epIndex, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse epIndex: %w", err)
	}

	return uint(bangumiID64), epIndex, nil
}

var (
	reDashNumber  = regexp.MustCompile(`-\s+(\d{1,3})`)
	reBracketNum  = regexp.MustCompile(`\[(\d{1,3})\]`)
	reEP          = regexp.MustCompile(`(?i)EP(\d{1,3})`)
	reE           = regexp.MustCompile(`(?i)E(\d{1,3})`)
	reChinese     = regexp.MustCompile(`第(\d{1,3})`)
	reSxxExx      = regexp.MustCompile(`(?i)S\d+E(\d{1,3})`)
)

// ExtractEpIndexFromFilename 从文件名中提取集数
// 支持格式: E01, EP01, 第01集, - 01, [01], S01E01 等
func ExtractEpIndexFromFilename(filename string) int {
	dotIndex := strings.LastIndex(filename, ".")
	if dotIndex > 0 {
		filename = filename[:dotIndex]
	}

	if num := matchAndExtract(reDashNumber, filename); num > 0 {
		return num
	}
	if num := matchAndExtract(reBracketNum, filename); num > 0 {
		return num
	}
	if num := matchAndExtract(reEP, filename); num > 0 {
		return num
	}
	if num := matchAndExtract(reSxxExx, filename); num > 0 {
		return num
	}
	if num := matchAndExtract(reChinese, filename); num > 0 {
		return num
	}
	if num := extractEWithCheck(filename); num > 0 {
		return num
	}

	return extractTrailingNumber(filename)
}

func matchAndExtract(re *regexp.Regexp, s string) int {
	matches := re.FindStringSubmatch(s)
	if len(matches) > 1 {
		num, err := strconv.Atoi(matches[1])
		if err == nil && num > 0 && num < 1000 {
			return num
		}
	}
	return 0
}

func extractEWithCheck(filename string) int {
	idxs := reE.FindAllStringIndex(filename, -1)
	for _, idx := range idxs {
		start := idx[0]
		if start > 0 {
			prev2 := strings.ToUpper(filename[max(0, start-2) : start])
			if strings.HasSuffix(prev2, "HV") || strings.HasSuffix(prev2, "EC") {
				continue
			}
		}
		after := filename[idx[1]:]
		numStr := ""
		for _, ch := range after {
			if ch >= '0' && ch <= '9' {
				numStr += string(ch)
			} else {
				break
			}
		}
		if numStr != "" {
			num, err := strconv.Atoi(numStr)
			if err == nil && num > 0 && num < 1000 {
				return num
			}
		}
	}
	return 0
}

func extractTrailingNumber(filename string) int {
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] >= '0' && filename[i] <= '9' {
			end := i + 1
			for i >= 0 && filename[i] >= '0' && filename[i] <= '9' {
				i--
			}
			numStr := filename[i+1 : end]
			num, err := strconv.Atoi(numStr)
			if err == nil && num > 0 && num < 1000 {
				return num
			}
			break
		}
		if (filename[i] >= 'A' && filename[i] <= 'Z') ||
			(filename[i] >= 'a' && filename[i] <= 'z') {
			break
		}
		ch := rune(filename[i])
		if ch >= 0x4e00 && ch <= 0x9fff {
			break
		}
	}
	return 1
}
