package workers

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

type DanmakuLine struct {
	Time  float64 `json:"time"`
	Text  string  `json:"text"`
	Color int     `json:"color"`
	Type  int     `json:"type"`
}

type danmakuRoot struct {
	XMLName xml.Name       `xml:"i"`
	Items   []danmakuEntry `xml:"d"`
}

type danmakuEntry struct {
	XMLName xml.Name `xml:"d"`
	Attr    string   `xml:"p,attr"`
	Text    string   `xml:",chardata"`
}

func parseDanDanXML(xmlData []byte) ([]DanmakuLine, error) {
	var root danmakuRoot
	if err := xml.Unmarshal(xmlData, &root); err != nil {
		return nil, fmt.Errorf("parse danmaku xml: %w", err)
	}

	lines := make([]DanmakuLine, 0, len(root.Items))
	for _, item := range root.Items {
		parts := strings.Split(item.Attr, ",")
		if len(parts) < 4 {
			continue
		}

		time, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			continue
		}

		danmakuType, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}

		color, err := strconv.Atoi(parts[3])
		if err != nil {
			continue
		}

		lines = append(lines, DanmakuLine{
			Time:  time,
			Text:  strings.TrimSpace(item.Text),
			Color: color,
			Type:  danmakuType,
		})
	}

	return lines, nil
}
